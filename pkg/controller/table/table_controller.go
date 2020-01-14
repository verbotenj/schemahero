/*
Copyright 2019 Replicated, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package table

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"
	databasesv1alpha3 "github.com/schemahero/schemahero/pkg/apis/databases/v1alpha3"
	schemasv1alpha3 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha3"
	databasesclientv1alpha3 "github.com/schemahero/schemahero/pkg/client/schemaheroclientset/typed/databases/v1alpha3"
	schemasclientv1alpha3 "github.com/schemahero/schemahero/pkg/client/schemaheroclientset/typed/schemas/v1alpha3"
	"github.com/schemahero/schemahero/pkg/logger"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Table Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTable{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("table-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Table
	err = c.Watch(&source.Kind{Type: &schemasv1alpha3.Table{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Add an informer on pods, which are created to deploy schemas. the informer will
	// update the status of the table custom resource and do a little garbage collection
	generatedClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())
	generatedInformers := kubeinformers.NewSharedInformerFactory(generatedClient, time.Minute)
	err = mgr.Add(manager.RunnableFunc(func(s <-chan struct{}) error {
		generatedInformers.Start(s)
		<-s
		return nil
	}))
	if err != nil {
		return err
	}

	// watch for pods because pods are how schemahero deploys, and the lifecycle of a pod is important
	// to ensure that we are deployed
	err = c.Watch(&source.Informer{
		Informer: generatedInformers.Core().V1().Pods().Informer(),
	}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileTable{}

// ReconcileTable reconciles a Table object
type ReconcileTable struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Table object and makes changes based on the state read
// and what is in the Table.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=schemas.schemahero.io,resources=tables,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=schemas.schemahero.io,resources=tables/status,verbs=get;update;patch
func (r *ReconcileTable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance, instanceErr := r.getInstance(request)
	if instanceErr == nil {
		result, err := r.reconcileInstance(instance)
		if err != nil {
			logger.Error(err)
		}
		return result, err
	}

	pod := &corev1.Pod{}
	podErr := r.Get(context.TODO(), request.NamespacedName, pod)
	if podErr == nil {
		result, err := r.reconcilePod(pod)
		if err != nil {
			logger.Error(err)
		}
		return result, err
	}

	return reconcile.Result{}, errors.New("unknown error in table reconciler")
}

func (r *ReconcileTable) reconcileInstance(instance *schemasv1alpha3.Table) (reconcile.Result, error) {
	logger.Debug("reconciling table",
		zap.String("kind", instance.Kind),
		zap.String("name", instance.Name),
		zap.String("database", instance.Spec.Database))

	database, err := r.getDatabaseSpec(instance.Namespace, instance.Spec.Database)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get database spec")
	}

	if database == nil {
		logger.Debug("requeuing table reconcile request for 10 seconds because database instance was not present",
			zap.String("database.name", instance.Spec.Database),
			zap.String("database.namespace", instance.Namespace))

		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second * 10,
		}, nil
	}

	matchingType := r.checkDatabaseTypeMatches(&database.Connection, instance.Spec.Schema)
	if !matchingType {
		return reconcile.Result{}, errors.New("unable to deploy table to connection of different type")
	}

	for _, plan := range instance.Status.Plans {
		if plan.ApprovedAt > 0 && plan.ExecutedAt == 0 {
			// execute!
			if err := r.deploy(database, instance, plan); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to deploy planned migration")
			}

			return reconcile.Result{}, nil
		}
	}

	// if there's already a plan, do nothing
	tableSHA, err := instance.GetSHA()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to get table sha")
	}

	for _, plan := range instance.Status.Plans {
		if plan.Name == tableSHA {
			return reconcile.Result{}, nil
		}
	}

	// Deploy the plan
	if err := r.plan(database, instance); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to schedule plan phase")
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTable) reconcilePod(pod *corev1.Pod) (reconcile.Result, error) {
	podLabels := pod.GetObjectMeta().GetLabels()
	role, ok := podLabels["schemahero-role"]
	if !ok {
		return reconcile.Result{}, nil
	}

	logger.Debug("reconciling schemahero pod",
		zap.String("kind", pod.Kind),
		zap.String("name", pod.Name),
		zap.String("role", role),
		zap.String("podPhase", string(pod.Status.Phase)))

	if role != "table" && role != "plan" {
		return reconcile.Result{}, nil
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		if role == "plan" {
			// Write the plan from stdout to the object itself
			cfg, err := config.GetConfig()
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to get config")
			}
			client, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to create client")
			}

			podLogOpts := corev1.PodLogOptions{}
			req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
			podLogs, err := req.Stream()
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to open log stream")
			}
			defer podLogs.Close()

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, podLogs)
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to copy logs too buffer")
			}

			out := buf.String()

			// remove empty lines from output
			// the planner plans each row, and can leave empty lines
			out = strings.Replace(out, "\n\n", "\n", -1)

			logger.Debug("read output from pod",
				zap.String("kind", pod.Kind),
				zap.String("name", pod.Name),
				zap.String("role", role),
				zap.String("output", out))

			tableName, ok := podLabels["schemahero-name"]
			if !ok {
				return reconcile.Result{}, nil
			}
			tableNamespace, ok := podLabels["schemahero-namespace"]
			if !ok {
				return reconcile.Result{}, nil
			}

			schemasClient, err := schemasclientv1alpha3.NewForConfig(cfg)
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to create schema client")
			}

			table, err := schemasClient.Tables(tableNamespace).Get(tableName, metav1.GetOptions{})
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to get existing table")
			}

			tableSHA, err := table.GetSHA()
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to get sha of table")
			}

			plan := schemasv1alpha3.TablePlan{
				Name:       tableSHA,
				DDL:        out,
				PlannedAt:  time.Now().Unix(),
				ApprovedAt: 0,
				RejectedAt: 0,
				ExecutedAt: 0,
			}
			table.Status.Plans = append(table.Status.Plans, &plan)

			logger.Debug("adding plan to table",
				zap.String("table", table.Name),
				zap.String("planName", tableSHA),
				zap.String("plan", out))

			_, err = schemasClient.Tables(table.Namespace).Update(table)
			if err != nil {
				return reconcile.Result{}, errors.Wrap(err, "failed to write plan to table status")
			}
		}

		// Delete the pod and config map
		if err := r.Delete(context.Background(), pod); err != nil {
			return reconcile.Result{}, err
		}

		// read the name of the config map
		configMapName := ""
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == "specs" && volume.ConfigMap != nil {
				configMapName = volume.ConfigMap.Name
			}
		}

		configMap := corev1.ConfigMap{}
		err := r.Get(context.Background(), types.NamespacedName{Name: configMapName, Namespace: pod.Namespace}, &configMap)
		if err != nil {
			return reconcile.Result{}, err
		}

		if err := r.Delete(context.Background(), &configMap); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileTable) getInstance(request reconcile.Request) (*schemasv1alpha3.Table, error) {
	v1alpha3instance := &schemasv1alpha3.Table{}
	err := r.Get(context.Background(), request.NamespacedName, v1alpha3instance)
	if err != nil {
		return nil, err // don't wrap
	}

	return v1alpha3instance, nil
}

func (r *ReconcileTable) getDatabaseSpec(namespace string, name string) (*databasesv1alpha3.Database, error) {
	logger.Debug("getting database spec",
		zap.String("namespace", namespace),
		zap.String("name", name))

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	databasesClient, err := databasesclientv1alpha3.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	database, err := databasesClient.Databases(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		// tables might be deployed before a database... if this is the case
		// we don't want to crash, we want to re-reconcile later
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	// try to parse the secret too, the database may be deployed, but that doesn't mean it's ready
	// TODO this would be better as a status field on the database object, instead of this leaky
	// interface
	if database.Connection.Postgres != nil {
		_, err := r.readConnectionURI(database.Namespace, database.Connection.Postgres.URI)
		if err != nil {
			return nil, nil
		}
	} else if database.Connection.Mysql != nil {
		_, err := r.readConnectionURI(database.Namespace, database.Connection.Mysql.URI)
		if err != nil {
			return nil, nil
		}
	}

	return database, nil
}

func (r *ReconcileTable) checkDatabaseTypeMatches(connection *databasesv1alpha3.DatabaseConnection, tableSchema *schemasv1alpha3.TableSchema) bool {
	if connection.Postgres != nil {
		return tableSchema.Postgres != nil
	} else if connection.Mysql != nil {
		return tableSchema.Mysql != nil
	}

	return false
}

func (r *ReconcileTable) plan(database *databasesv1alpha3.Database, table *schemasv1alpha3.Table) error {
	if err := r.ensureTableConfigMap(database, table, nil); err != nil {
		return err
	}

	if err := r.ensureTablePod(database, table, true); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileTable) deploy(database *databasesv1alpha3.Database, table *schemasv1alpha3.Table, plan *schemasv1alpha3.TablePlan) error {
	if err := r.ensureTableConfigMap(database, table, plan); err != nil {
		return err
	}

	if err := r.ensureTablePod(database, table, false); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileTable) ensureTableConfigMap(database *databasesv1alpha3.Database, table *schemasv1alpha3.Table, plan *schemasv1alpha3.TablePlan) error {
	var desiredConfigMap *corev1.ConfigMap

	if plan == nil {
		// we need to plan
		cm, err := r.planConfigMap(database, table)
		if err != nil {
			return errors.Wrap(err, "failed to get config map object")
		}

		desiredConfigMap = cm
	} else {
		// let's deploy
		cm, err := r.applyConfigMap(database, table, plan)
		if err != nil {
			return errors.Wrap(err, "failed to get config map object")
		}

		desiredConfigMap = cm
	}

	existingConfigMap := corev1.ConfigMap{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredConfigMap.Name, Namespace: desiredConfigMap.Namespace}, &existingConfigMap); err != nil {
		if kuberneteserrors.IsNotFound(err) {
			err = r.Create(context.TODO(), desiredConfigMap)
			if err != nil {
				return errors.Wrap(err, "failed to create configmap")
			}
		}

		return errors.Wrap(err, "failed to get existing configmap")
	}

	return nil
}

func (r *ReconcileTable) ensureTablePod(database *databasesv1alpha3.Database, table *schemasv1alpha3.Table, isPlan bool) error {
	var desiredPod *corev1.Pod

	if isPlan {
		p, err := r.planPod(database, table)
		if err != nil {
			return errors.Wrap(err, "failed to get pod object")
		}

		desiredPod = p
	} else {
		p, err := r.applyPod(database, table)
		if err != nil {
			return errors.Wrap(err, "failed to get pod object")
		}

		desiredPod = p
	}

	existingPod := corev1.Pod{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredPod.Name, Namespace: desiredPod.Namespace}, &existingPod); err != nil {
		if kuberneteserrors.IsNotFound(err) {
			err = r.Create(context.TODO(), desiredPod)
			if err != nil {
				return errors.Wrap(err, "failed to create table migration pod")
			}

			return nil
		}

		return errors.Wrap(err, "failed to get existing pod object")
	}

	return nil
}

func (r *ReconcileTable) readConnectionURI(namespace string, valueOrValueFrom databasesv1alpha3.ValueOrValueFrom) (string, error) {
	if valueOrValueFrom.Value != "" {
		return valueOrValueFrom.Value, nil
	}

	if valueOrValueFrom.ValueFrom == nil {
		return "", errors.New("value and valueFrom cannot both be nil/empty")
	}

	if valueOrValueFrom.ValueFrom.SecretKeyRef != nil {
		secret := &corev1.Secret{}
		secretNamespacedName := types.NamespacedName{
			Name:      valueOrValueFrom.ValueFrom.SecretKeyRef.Name,
			Namespace: namespace,
		}

		if err := r.Get(context.Background(), secretNamespacedName, secret); err != nil {
			if kuberneteserrors.IsNotFound(err) {
				return "", errors.New("table secret not found")
			} else {
				return "", errors.Wrap(err, "failed to get existing connection secret")
			}
		}

		return string(secret.Data[valueOrValueFrom.ValueFrom.SecretKeyRef.Key]), nil
	}

	return "", errors.New("unable to find supported valueFrom")
}

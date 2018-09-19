/*

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

package dynamickubelet

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	dynamickubeletv1alpha1 "github.com/rphillips/dynamic-kubelet-controller/pkg/apis/dynamickubelet/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var errorNodeNotManaged = errors.New("Node not managed")
var nodeRolesToConfigMap = map[string]string{
	"node-role.kubernetes.io/master": "node-config-master",
	"node-role.kubernetes.io/worker": "node-config-worker",
}
var defaultConfigMapOptions = map[string]string{
	"serializeImagePulls": "true",
}

const (
	customNodeLabel  = "node-role.kubernetes.io/custom"
	defaultNamespace = "openshift-node"
)

func getConfigMapName(node *corev1.Node) (types.NamespacedName, error) {
	if configMap, ok := node.Labels[customNodeLabel]; ok {
		namespace := defaultNamespace
		name := configMap
		if namespaceName := strings.Split(configMap, "/"); len(namespaceName) == 2 {
			namespace = namespaceName[0]
			name = namespaceName[1]
		}
		return types.NamespacedName{Namespace: namespace, Name: name}, nil
	}
	for nodeRole, configMap := range nodeRolesToConfigMap {
		if _, ok := node.Labels[nodeRole]; ok {
			return types.NamespacedName{Namespace: defaultNamespace, Name: configMap}, nil
		}
	}
	return types.NamespacedName{}, errorNodeNotManaged
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DynamicKubelet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this dynamickubelet.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDynamicKubelet{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("dynamickubelet-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// Watch for changes to DynamicKubelet
	err = c.Watch(&source.Kind{Type: &dynamickubeletv1alpha1.DynamicKubelet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileDynamicKubelet{}

// ReconcileDynamicKubelet reconciles a DynamicKubelet object
// +kubebuilder:informers:group=core,version=v1,kind=Node
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;patch;update;
type ReconcileDynamicKubelet struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DynamicKubelet object and makes changes based on the state read
// and what is in the DynamicKubelet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:informers:group=core,version=v1,kind=Node
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=dynamickubelet.openshift.io,resources=dynamickubelets,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileDynamicKubelet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("Got event %v", request.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name}, node); err != nil {
		return reconcile.Result{}, err
	}
	configMapName, err := getConfigMapName(node)
	if err != nil {
		return reconcile.Result{}, err
	}
	if node.Spec.ConfigSource != nil && node.Spec.ConfigSource.ConfigMap.Name == configMapName.Name && node.Spec.ConfigSource.ConfigMap.Namespace == configMapName.Namespace {
		log.Printf("Already Added ConfigMap %v/%v to Node %v", configMapName.Namespace, configMapName.Name, node.Name)
		return reconcile.Result{}, nil
	}
	configMap := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, configMapName, configMap); err != nil {
		if apierrs.IsNotFound(err) {
			err = r.createDefaultConfigMap(ctx, configMapName)
			if err != nil {
				return reconcile.Result{}, err
			}
		} else {
			return reconcile.Result{}, err
		}
	}
	err = r.addConfigMap(ctx, node, configMapName)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Printf("... added ConfigMap %v/%v to Node %v", configMapName.Namespace, configMapName.Name, node.Name)
	return reconcile.Result{}, nil
}

func (r *ReconcileDynamicKubelet) createDefaultConfigMap(ctx context.Context, nsName types.NamespacedName) error {
	log.Printf("Creating default ConfigMap %v/%v", nsName.Namespace, nsName.Name)
	cm := &corev1.ConfigMap{}
	cm.Name = nsName.Name
	cm.Namespace = nsName.Namespace
	cm.Data = defaultConfigMapOptions
	return r.Client.Create(ctx, cm)
}

func (r *ReconcileDynamicKubelet) addConfigMap(ctx context.Context, node *corev1.Node, nsName types.NamespacedName) error {
	log.Printf("Adding ConfigMap %v/%v to Node %v", nsName.Namespace, nsName.Name, node.Name)
	node.Spec.ConfigSource = &corev1.NodeConfigSource{
		ConfigMap: &corev1.ConfigMapNodeConfigSource{
			Namespace:        nsName.Namespace,
			Name:             nsName.Name,
			KubeletConfigKey: "kubelet",
		},
	}
	return r.Client.Update(ctx, node)
}

/*
Copyright 2022.

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

package controllers

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Node object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	node := v1.Node{}
	r.Client.Get(context.TODO(), req.NamespacedName, &node)

	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	cpuAllocatable := node.Status.Allocatable.Cpu()
	memAllocatable := node.Status.Allocatable.Memory()
	needUpdtae := false
	if node.Annotations["cpuAllocatable"] == "" {
		needUpdtae = true
		node.Annotations["cpuAllocatable"] = cpuAllocatable.String()
	} else {
		if node.Annotations["cpuAllocatable"] == cpuAllocatable.String() {
			needUpdtae = true
		}
	}

	if node.Annotations["memAllocatable"] == "" {
		needUpdtae = true
		node.Annotations["memAllocatable"] = memAllocatable.String()
	} else {
		if node.Annotations["memAllocatable"] == memAllocatable.String() {
			needUpdtae = true
		}
	}

	multipleStr := os.Getenv("multiple")

	multiple,_ := strconv.Atoi(multipleStr)
	if multiple == 0{
		multiple = 4
	}

	if needUpdtae {
		klog.V(5).Infof("Need update: %s",node.Name)
		cpuAllocatable.Set(cpuAllocatable.Value() * int64(multiple))
		node.Status.Allocatable["cpu"] = *cpuAllocatable
		node.Status.Capacity["cpu"] = *cpuAllocatable
		klog.V(5).Infof("scale: cpu %s -> %s",node.Annotations["cpuAllocatable"], cpuAllocatable.String())

		memAllocatable.Set(memAllocatable.Value() * int64(multiple))
		node.Status.Allocatable["memory"] = *memAllocatable
		node.Status.Capacity["memory"] = *memAllocatable
		klog.V(5).Infof("scale: memory %s -> %s",node.Annotations["memAllocatable"], memAllocatable.String())

		r.Client.Status().Update(context.TODO(), &node)
		//if err != nil {
		//	return ctrl.Result{}, err
		//}
	} else {
		klog.V(5).Infof("No need update: %s",node.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Node{}).
		Complete(r)
}

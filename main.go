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

package main

import (
	"context"
	"dev-env-controller/api/object"
	"flag"
	"fmt"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8s_scheme "sigs.k8s.io/controller-runtime/pkg/scheme"
	"dev-env-controller/controllers"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	SyncPeriod := time.Duration(time.Second * 20)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		SyncPeriod:             &SyncPeriod,
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "fc4c6eb5.my.domain",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	AddToScheme()

	if err = (&controllers.NodeReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}

	for _, obj := range object.ObjMap {
		clusterReconciler := &controllers.ClusterReconciler{
			Client:           mgr.GetClient(),
			Log:              ctrl.Log.WithName("controllers").WithName(obj.Plural),
			Scheme:           mgr.GetScheme(),
			Object:           obj,
			Dependence:       &map[string][]string{},
		}
		if err := clusterReconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", obj.Plural)
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}


// add middleware scheme
func AddToScheme() error {
	cs, err := clientset.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("failed to get k8s client: %v", err)
	}
	crdList, err := cs.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), v1.ListOptions{LabelSelector: "MiddlewareCluster"})
	if err != nil {
		return fmt.Errorf("query crds error: %v", err)
	}


	for _, crd := range crdList.Items {
		versionNum := len(crd.Spec.Versions)
		groupVersion := schema.GroupVersion{Group: crd.Spec.Group, Version: crd.Spec.Versions[versionNum-1].Name}
		schemeBuilder := &k8s_scheme.Builder{GroupVersion: groupVersion}
		schemeBuilder.AddToScheme(scheme)

		var objOne = &unstructured.Unstructured{}
		gvk := schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: crd.Spec.Versions[versionNum-1].Name,
			Kind:    crd.Spec.Names.Kind,
		}
		objOne.SetGroupVersionKind(gvk)

		var objList = &unstructured.UnstructuredList{}
		gvkList := schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: crd.Spec.Versions[versionNum-1].Name,
			Kind:    crd.Spec.Names.ListKind,
		}
		objList.SetGroupVersionKind(gvkList)

		cObject := &object.CustomObject{}

		if value, ok := object.ObjMap[crd.Spec.Names.Singular]; ok {
			cObject = value
		}
		cObject.One = objOne
		cObject.List = objList
		cObject.Plural = crd.Spec.Names.Plural

		// add status mapping from annotations
		annotations := crd.GetAnnotations()
		statusMapping := make(map[string]string)
		for key, value := range annotations {
			if strings.HasPrefix(key,"harmonycloud"){
				statusMapping[key] = value
			}
		}
		if len(statusMapping) > 0 {
			cObject.StatusMapping = statusMapping
		}

		object.ObjMap[crd.Spec.Names.Singular] = cObject
	}
	return nil
}
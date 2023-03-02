/*
Copyright 2021.

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
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"dev-env-controller/api/object"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sigs.k8s.io/yaml"
	"strings"
	"time"

	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-logr/logr"
	"github.com/sergi/go-diff/diffmatchpatch"

	v1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	Object           *object.CustomObject
	Dependence       *map[string][]string
}

//+kubebuilder:rbac:groups=es.middleware.hc.cn.harmonycloud.cn,resources=esclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=es.middleware.hc.cn.harmonycloud.cn,resources=esclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=es.middleware.hc.cn.harmonycloud.cn,resources=esclusters/finalizers,verbs=update


var objectCache sync.Map

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues(r.Object.One.GetKind(), req.NamespacedName)
	logger.V(4).Info("Reconcile")

	var objOne = &unstructured.Unstructured{}
	objOne.SetGroupVersionKind(r.Object.One.GetObjectKind().GroupVersionKind())


	if err := r.Client.Get(context.TODO(), req.NamespacedName, objOne); err != nil {
		if k8sErrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	objectData := map[string]interface{}{}
	objectData["spec"] = objOne.Object["spec"]
	objectData["status"] = objOne.Object["status"]
	annotations, _,_ := unstructured.NestedMap(objOne.Object,"metadata","annotations")
	objectData["annotations"] = annotations

	oldObject, ok := objectCache.Load(req.String())
	if ok{
		oldObjectHash := md5Interface(oldObject)
		objectHash := md5Interface(objectData)
		if oldObjectHash != objectHash{
			marshalobject,_ :=  yaml.Marshal(objectData)
			marshalOldObject,_ :=  yaml.Marshal(oldObject)

			json.Marshal(objectHash)
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain( string(marshalOldObject), string(marshalobject),false)

			dir := os.Getenv("OUT_DIR")
			if dir == ""{
				dir = "/tmp"
			}
			filePath := fmt.Sprintf("%s/%s_%s_%s",dir,req.Namespace,strings.ToLower(objOne.GetKind()),req.Name)
			file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println("文件打开失败", err)
			}
			defer file.Close()
			write := bufio.NewWriter(file)

			write.WriteString(fmt.Sprintf("====%s==%s========\r\n",req.String(),time.Now().String()))
			write.WriteString(DiffPrettyText(diffs) + "\r\n" )

			write.Flush()
			objectCache.Store(req.String(),objectData)
		}
	}else{
		objectCache.Store(req.String(),objectData)
	}
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, Children := range r.Object.Children {
		err := ctrl.NewControllerManagedBy(mgr).
			For(Children.One).
			Complete(r)
		if err != nil {
			return err
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(r.Object.One).
		Owns(&v1.StatefulSet{}).
		Complete(r)
}

func md5Interface(origin interface{}) string {
	m := md5.New()
	byteData, _ := json.Marshal(origin)
	m.Write(byteData)
	return hex.EncodeToString(m.Sum(nil))
}

func DiffPrettyText(diffs []diffmatchpatch.Diff) string {
	var buff bytes.Buffer

	for _, diff := range diffs {
		text := diff.Text

		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			_, _ = buff.WriteString("\x1b[32m")
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString("\x1b[0m")
		case diffmatchpatch.DiffDelete:
			_, _ = buff.WriteString("\x1b[31m")
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString("\x1b[0m")
		case diffmatchpatch.DiffEqual:
			// 单行直接打印
			if strings.Index(text,"\n") < 0{
				_, _ = buff.WriteString(text)
			}else{
				// 保留最后一行
				index := strings.Index(text,"\n")
				if  index >=0 {
					_, _ = buff.WriteString(text[:index + 1])
				}

				// 保留第一行
				index = strings.LastIndex(text,"\n")
				if index >= 1{
					_, _ = buff.WriteString(text[index + 1:])
				}
			}
		}
	}

	return buff.String()
}
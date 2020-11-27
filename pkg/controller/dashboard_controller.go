// Copyright (c) 2020 Red Hat, Inc.

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/open-cluster-management/grafana-dashboard-loader/pkg/util"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
)

// DashboardLoader ...
type DashboardLoader struct {
	coreClient corev1client.CoreV1Interface
	informer   cache.SharedIndexInformer
}

var (
	grafanaURI = "http://127.0.0.1:3001"
	//retry on errors
	retry = 10
)

// RunGrafanaDashboardController ...
func RunGrafanaDashboardController(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	// Build kubclient client and informer for managed cluster
	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	go newKubeInformer(kubeClient.CoreV1()).Run(ctx.Done())
	<-ctx.Done()
	return nil
}

func newKubeInformer(coreClient corev1client.CoreV1Interface) cache.SharedIndexInformer {
	// get watched namespace
	watchedNS := os.Getenv("POD_NAMESPACE")

	kubeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return coreClient.ConfigMaps(watchedNS).List(context.TODO(), metav1.ListOptions{
					LabelSelector: "grafana-custom-dashboard=true",
				})
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return coreClient.ConfigMaps(watchedNS).Watch(context.TODO(), metav1.ListOptions{
					LabelSelector: "grafana-custom-dashboard=true",
				})
			},
		},
		&corev1.ConfigMap{}, time.Second, cache.Indexers{},
	)

	kubeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("detect there is a new dashboard %v created", obj.(*corev1.ConfigMap).Name)
			updateDashboard(obj, false)
		},
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old.(*corev1.ConfigMap).Data, new.(*corev1.ConfigMap).Data) {
				klog.Infof("detect there is a customized dashboard %v updated", new.(*corev1.ConfigMap).Name)
				updateDashboard(new, false)
			}
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("detect there is a customized dashboard %v deleted", obj.(*corev1.ConfigMap).Name)
			deleteDashboard(obj.(*corev1.ConfigMap).Name, obj.(*corev1.ConfigMap).Namespace)
		},
	})

	return kubeInformer
}

func hasCustomFolder() float64 {
	grafanaURL := grafanaURI + "/api/folders"
	body, _ := util.SetRequest("GET", grafanaURL, nil, retry)

	folders := []map[string]interface{}{}
	err := json.Unmarshal(body, &folders)
	if err != nil {
		klog.Error("Failed to unmarshall response body", "error", err)
		return 0
	}

	for _, folder := range folders {
		if folder["title"] == "Custom" {
			return folder["id"].(float64)
		}
	}
	return 0
}

func createCustomFolder() float64 {
	folderID := hasCustomFolder()
	if folderID == 0 {
		grafanaURL := grafanaURI + "/api/folders"
		body, _ := util.SetRequest("POST", grafanaURL, strings.NewReader("{\"title\":\"Custom\"}"), retry)

		folder := map[string]interface{}{}
		err := json.Unmarshal(body, &folder)
		if err != nil {
			klog.Error("Failed to unmarshall response body", "error", err)
			return 0
		}
		return folder["id"].(float64)
	}
	return folderID
}

// updateDashboard is used to update the customized dashboards via calling grafana api
func updateDashboard(obj interface{}, overwrite bool) {

	folderID := 0.0
	labels := obj.(*corev1.ConfigMap).ObjectMeta.Labels
	if labels["general-folder"] == "" || strings.ToLower(labels["general-folder"]) != "true" {
		folderID = createCustomFolder()
		if folderID == 0 {
			klog.Error("Failed to get custom folder id")
			return
		}
	}
	for _, value := range obj.(*corev1.ConfigMap).Data {

		dashboard := map[string]interface{}{}
		err := json.Unmarshal([]byte(value), &dashboard)
		if err != nil {
			klog.Error("Failed to unmarshall data", "error", err)
			return
		}
		dashboard["uid"], _ = util.GenerateUID(obj.(*corev1.ConfigMap).GetName(),
			obj.(*corev1.ConfigMap).GetNamespace())
		dashboard["id"] = nil
		data := map[string]interface{}{
			"folderId":  folderID,
			"overwrite": overwrite,
			"dashboard": dashboard,
		}

		b, err := json.Marshal(data)
		if err != nil {
			klog.Error("failed to marshal body", "error", err)
			return
		}

		grafanaURL := grafanaURI + "/api/dashboards/db"
		body, respStatusCode := util.SetRequest("POST", grafanaURL, bytes.NewBuffer(b), retry)

		if respStatusCode != http.StatusOK {
			if respStatusCode == http.StatusPreconditionFailed {
				if strings.Contains(string(body), "version-mismatch") {
					updateDashboard(obj, true)
				} else if strings.Contains(string(body), "name-exists") {
					klog.Info("the dashboard name already existed")
				} else {
					klog.Infof("failed to create/update: %v", respStatusCode)
				}
			} else {
				klog.Infof("failed to create/update: %v", respStatusCode)
			}
		} else {
			klog.Info("Dashboard created/updated")
		}
	}

}

// DeleteDashboard ...
func deleteDashboard(name, namespace string) {
	uid, _ := util.GenerateUID(name, namespace)
	grafanaURL := grafanaURI + "/api/dashboards/uid/" + uid

	_, respStatusCode := util.SetRequest("DELETE", grafanaURL, nil, retry)
	if respStatusCode != http.StatusOK {
		klog.Errorf("failed to delete dashboard %v with %v", name, respStatusCode)
	} else {
		klog.Info("Dashboard deleted")
	}
	return
}

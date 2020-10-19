// Copyright (c) 2020 Red Hat, Inc.

package loader

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
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/open-cluster-management/grafana-dashboard-loader/pkg/util"
)

// DashboardLoader ...
type DashboardLoader struct {
	clientset *kubernetes.Clientset
	informer  cache.SharedIndexInformer
}

const (
	grafanaURI = "http://127.0.0.1:3001"
)

// NewDashboardLoader ...
func NewDashboardLoader(clientset *kubernetes.Clientset) *DashboardLoader {
	return &DashboardLoader{clientset: clientset}
}

// WatchDashboardConfigMaps ...
func (loader *DashboardLoader) WatchDashboardConfigMaps() {

	watchedNS := os.Getenv("POD_NAMESPACE")

	loader.informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				return loader.clientset.CoreV1().ConfigMaps(watchedNS).List(context.TODO(), metav1.ListOptions{
					LabelSelector: "grafana-custom-dashboard=true",
				})
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return loader.clientset.CoreV1().ConfigMaps(watchedNS).Watch(context.TODO(), metav1.ListOptions{
					LabelSelector: "grafana-custom-dashboard=true",
				})
			},
		},
		&corev1.ConfigMap{}, time.Second, cache.Indexers{},
	)

	loader.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("detect there is a new dashboard created", "name", obj.(*corev1.ConfigMap).Name)
			loader.updateDashboard(obj, false)
		},
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old.(*corev1.ConfigMap).Data, new.(*corev1.ConfigMap).Data) {
				klog.Info("detect there is a customized dashboard updated", "name", new.(*corev1.ConfigMap).Name)
				loader.updateDashboard(new, false)
			}
		},
		DeleteFunc: func(obj interface{}) {
			klog.Info("detect there is a customized dashboard deleted", "name", obj.(*corev1.ConfigMap).Name)
			loader.deleteDashboard(obj.(*corev1.ConfigMap).Name, obj.(*corev1.ConfigMap).Namespace)
		},
	})
}

// Run ...
func (loader *DashboardLoader) Run(stop <-chan struct{}) {

	go loader.informer.Run(stop)

	for {
		time.Sleep(time.Second * 30)
	}
	<-stop
	klog.Info("loader terminated")
}

func (loader *DashboardLoader) hasCustomFolder() float64 {
	grafanaURL := grafanaURI + "/api/folders"

	body, _ := util.SetRequest("GET", grafanaURL, nil)

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

func (loader *DashboardLoader) createCustomFolder() float64 {
	folderID := loader.hasCustomFolder()
	if folderID == 0 {
		grafanaURL := grafanaURI + "/api/folders"
		body, _ := util.SetRequest("POST", grafanaURL, strings.NewReader("{\"title\":\"Custom\"}"))

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

// UpdateDashboard is used to update the customized dashboards via calling grafana api
func (loader *DashboardLoader) updateDashboard(obj interface{}, overwrite bool) {

	folderID := 0.0
	labels := obj.(*corev1.ConfigMap).ObjectMeta.Labels
	if labels["general-folder"] == "" || strings.ToLower(labels["general-folder"]) != "true" {
		folderID = loader.createCustomFolder()
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
		dashboard["uid"] = util.GenerateUID(obj.(*corev1.ConfigMap).GetName(), obj.(*corev1.ConfigMap).GetNamespace())
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
		body, respStatusCode := util.SetRequest("POST", grafanaURL, bytes.NewBuffer(b))

		if respStatusCode != http.StatusOK {
			if respStatusCode == http.StatusPreconditionFailed {
				if strings.Contains(string(body), "version-mismatch") {
					loader.updateDashboard(obj, true)
				} else if strings.Contains(string(body), "name-exists") {
					klog.Info("the dashboard name already existed")
				} else {
					klog.Info("failed to create/update:", "", respStatusCode)
				}
			} else {
				klog.Info("failed to create/update: ", "", respStatusCode)
			}
		} else {
			klog.Info("Dashboard created/updated")
		}
	}

}

// DeleteDashboard ...
func (loader *DashboardLoader) deleteDashboard(name, namespace string) {
	uid := util.GenerateUID(name, namespace)
	grafanaURL := grafanaURI + "/api/dashboards/uid/" + uid

	util.SetRequest("DELETE", grafanaURL, nil)
	return
}

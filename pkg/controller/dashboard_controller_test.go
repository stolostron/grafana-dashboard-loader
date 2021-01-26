// Copyright (c) 2021 Red Hat, Inc.

package controller

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createDashboard() (*corev1.ConfigMap, error) {
	// read the whole file at once
	data, err := ioutil.ReadFile("../../examples/k8s-dashboard.yaml")
	if err != nil {
		panic(err)
	}

	var cm corev1.ConfigMap
	err = yaml.Unmarshal(data, &cm)
	return &cm, err
}

func createFakeServer(t *testing.T) {
	server3001 := http.NewServeMux()

	server3001.HandleFunc("/api/folders",
		func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("[{\"id\": 1,\"uid\": \"test\",\"title\": \"Custom\"}]"))
		},
	)
	server3001.HandleFunc("/api/dashboards/db",
		func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("done"))
		},
	)

	server3001.HandleFunc("/api/dashboards/uid/ff635a025bcfea7bc3dd4f508990a3e8",
		func(w http.ResponseWriter, req *http.Request) {
			w.Write([]byte("done"))
		},
	)

	err := http.ListenAndServe(":3001", server3001)
	if err != nil {
		t.Fatal("fail to create internal server at 3001")
	}
}

func TestGrafanaDashboardController(t *testing.T) {

	coreClient := fake.NewSimpleClientset().CoreV1()
	stop := make(chan struct{})

	go createFakeServer(t)
	retry = 1

	os.Setenv("POD_NAMESPACE", "ns2")

	informer := newKubeInformer(coreClient)
	go informer.Run(stop)

	cm, err := createDashboard()
	if err == nil {
		_, err := coreClient.ConfigMaps("ns2").Create(context.TODO(), cm, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("fail to create configmap with %v", err)
		}
		// wait for 2 second to trigger AddFunc of informer
		time.Sleep(time.Second * 2)

		cm.Data = map[string]string{}
		_, err = coreClient.ConfigMaps("ns2").Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("fail to update configmap with %v", err)
		}
		// wait for 2 second to trigger UpdateFunc of informer
		time.Sleep(time.Second * 2)

		cm, _ := createDashboard()
		_, err = coreClient.ConfigMaps("ns2").Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("fail to update configmap with %v", err)
		}

		// wait for 2 second to trigger UpdateFunc of informer
		time.Sleep(time.Second * 2)

		coreClient.ConfigMaps("ns2").Delete(context.TODO(), cm.GetName(), metav1.DeleteOptions{})
		time.Sleep(time.Second * 2)
	}

	close(stop)
	<-stop
}

package main

import (
	"context"
	"flag"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"
	"fmt"
  "net/http"
	"encoding/json"
	"log"
)

var (
	client *clientset.Clientset
	duckOrGoose string = "duck"
	leader string
)

func getNewLock(lockname, podname, namespace string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}
}

func doStuff() {
	for {
		klog.Info("doing stuff...")
		time.Sleep(5 * time.Second)
	}
}

func runLeaderElection(lock *resourcelock.LeaseLock, ctx context.Context, id string) {
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				klog.Info("im the leader")
				duckOrGoose = "goose"
  			http.ListenAndServe(":3000", nil)
			},
			OnStoppedLeading: func() {
				klog.Info("no longer the leader, staying inactive.")
			},
			OnNewLeader: func(current_id string) {
				leader = current_id
				if current_id == id {
					klog.Info("still the leader!")
					return
				}
				klog.Info("new leader is %s", current_id)
  			http.ListenAndServe(":3000", nil)
			},
		},
	})
}

func homePage(w http.ResponseWriter, r *http.Request) {
  fmt.Fprintf(w, "My Sleeper API" + duckOrGoose + " " + os.Getenv("POD_NAME"))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = "My Sleeper API says I'm a " + duckOrGoose + "!!! "
	resp["podName"] = os.Getenv("POD_NAME")
	resp["podLeader"] = leader
	var isGoose bool = true
	if duckOrGoose == "duck"  {
	  isGoose = false
	}
	resp["isGoose"] = fmt.Sprintf("%t",isGoose)

	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)
}

func setupRoutes() {
  http.HandleFunc("/", handleRequest)
}

func main() {

  fmt.Println("Go Web App Started on Port 3000")
  setupRoutes()

	var (
		leaseLockName      string
		leaseLockNamespace string
		podName            = os.Getenv("POD_NAME")
	)
	flag.StringVar(&leaseLockName, "lease-name", "", "Name of lease lock")
	flag.StringVar(&leaseLockNamespace, "lease-namespace", "default", "Name of lease lock namespace")
	flag.Parse()

	if leaseLockName == "" {
		klog.Fatal("missing lease-name flag")
	}
	if leaseLockNamespace == "" {
		klog.Fatal("missing lease-namespace flag")
	}

	config, err := rest.InClusterConfig()
	client = clientset.NewForConfigOrDie(config)

	if err != nil {
		klog.Fatalf("failed to get kubeconfig")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lock := getNewLock(leaseLockName, podName, leaseLockNamespace)
	runLeaderElection(lock, ctx, podName)
}

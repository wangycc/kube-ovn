package controller

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alauda/kube-ovn/pkg/ovs"
	"github.com/alauda/kube-ovn/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

// InitDefaultLogicalSwitch int the default logical switch for ovn network
func InitDefaultLogicalSwitch(config *Configuration) error {
	namespace := os.Getenv("KUBE_NAMESPACE")
	if namespace == "" {
		klog.Errorf("env KUBE_NAMESPACE not exists")
		return fmt.Errorf("env KUBE_NAMESPACE not exists")
	}

	ns, err := config.KubeClient.CoreV1().Namespaces().Get(namespace, v1.GetOptions{})
	if err != nil {
		return err
	}

	patchPayloadTemplate :=
		`[{
        "op": "%s",
        "path": "/metadata/annotations",
        "value": %s
    }]`
	payload := map[string]string{
		util.LogicalSwitchAnnotation: config.DefaultLogicalSwitch,
		util.CidrAnnotation:          config.DefaultCIDR,
		util.GatewayAnnotation:       config.DefaultGateway,
		util.ExcludeIpsAnnotation:    config.DefaultExcludeIps,
	}
	raw, _ := json.Marshal(payload)
	op := "replace"
	if len(ns.Annotations) == 0 {
		op = "add"
	}
	patchPayload := fmt.Sprintf(patchPayloadTemplate, op, raw)
	_, err = config.KubeClient.CoreV1().Namespaces().Patch(namespace, types.JSONPatchType, []byte(patchPayload))
	if err != nil {
		klog.Errorf("patch namespace %s failed %v", namespace, err)
	}
	return err
}

// InitNodeSwitch init node switch to connect host and pod
func InitNodeSwitch(config *Configuration) error {
	client := ovs.NewClient(config.OvnNbHost, config.OvnNbPort, "", 0, config.ClusterRouter, config.ClusterTcpLoadBalancer, config.ClusterUdpLoadBalancer, config.NodeSwitch, config.NodeSwitchCIDR)
	ss, err := client.ListLogicalSwitch()
	if err != nil {
		return err
	}
	klog.Infof("exists switches %v", ss)
	for _, s := range ss {
		if config.NodeSwitch == s {
			return nil
		}
	}

	err = client.CreateLogicalSwitch(config.NodeSwitch, config.NodeSwitchCIDR, config.NodeSwitchGateway, config.NodeSwitchGateway)
	if err != nil {
		return err
	}
	return nil
}

// InitClusterRouter init cluster router to connect different logical switches
func InitClusterRouter(config *Configuration) error {
	client := ovs.NewClient(config.OvnNbHost, config.OvnNbPort, "", 0, config.ClusterRouter, config.ClusterTcpLoadBalancer, config.ClusterUdpLoadBalancer, config.NodeSwitch, config.NodeSwitchCIDR)
	lrs, err := client.ListLogicalRouter()
	if err != nil {
		return err
	}
	klog.Infof("exists routers %v", lrs)
	for _, r := range lrs {
		if config.ClusterRouter == r {
			return nil
		}
	}
	return client.CreateLogicalRouter(config.ClusterRouter)
}

// InitLoadBalancer init the default tcp and udp cluster loadbalancer
func InitLoadBalancer(config *Configuration) error {
	client := ovs.NewClient(config.OvnNbHost, config.OvnNbPort, "", 0, config.ClusterRouter, config.ClusterTcpLoadBalancer, config.ClusterUdpLoadBalancer, config.NodeSwitch, config.NodeSwitchCIDR)
	tcpLb, err := client.FindLoadbalancer(config.ClusterTcpLoadBalancer)
	if err != nil {
		return fmt.Errorf("failed to find tcp lb %v", err)
	}
	if tcpLb == "" {
		klog.Infof("init cluster tcp load balancer %s", config.ClusterTcpLoadBalancer)
		err := client.CreateLoadBalancer(config.ClusterTcpLoadBalancer, util.ProtocolTCP)
		if err != nil {
			klog.Errorf("failed to crate cluster tcp load balancer %v", err)
			return err
		}
	} else {
		klog.Infof("tcp load balancer %s exists", tcpLb)
	}

	udpLb, err := client.FindLoadbalancer(config.ClusterUdpLoadBalancer)
	if err != nil {
		return fmt.Errorf("failed to find udp lb %v", err)
	}
	if udpLb == "" {
		klog.Infof("init cluster udp load balancer %s", config.ClusterUdpLoadBalancer)
		err := client.CreateLoadBalancer(config.ClusterUdpLoadBalancer, util.ProtocolUDP)
		if err != nil {
			klog.Errorf("failed to crate cluster udp load balancer %v", err)
			return err
		}
	} else {
		klog.Infof("udp load balancer %s exists", udpLb)
	}
	return nil
}

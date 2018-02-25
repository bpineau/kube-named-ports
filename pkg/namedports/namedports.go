// https://godoc.org/cloud.google.com/go
// https://godoc.org/google.golang.org/api/container/v1
// https://godoc.org/google.golang.org/api/compute/v1#InstanceGroupsSetNamedPortsRequest
// https://cloud.google.com/compute/docs/reference/rest/beta/instanceGroups/list
// https://github.com/golang/oauth2/blob/master/google/example_test.go

package namedports

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/api/compute/v0.beta"
	container "google.golang.org/api/container/v1"
)

func launch(project string, zone string) {
	ctx := context.Background()

	// See https://cloud.google.com/docs/authentication/.
	// Use GOOGLE_APPLICATION_CREDENTIALS environment variable to specify
	// a service account key file to authenticate to the API.

	hc, err := google.DefaultClient(ctx, container.CloudPlatformScope, compute.CloudPlatformScope)
	if err != nil {
		log.Fatalf("Could not get authenticated client: %v", err)
	}

	svc, err := container.New(hc)
	if err != nil {
		log.Fatalf("Could not initialize gke client: %v", err)
	}

	csvc, err := compute.New(hc)
	if err != nil {
		log.Fatalf("Could not initialize compute client: %v", err)
	}

	if err := setNamedPorts(svc, csvc, *project, *zone); err != nil {
		log.Fatal(err)
	}
}

func setNamedPorts(svc *container.Service, csvc *compute.Service, project, zone string) error {
	list, err := svc.Projects.Zones.Clusters.List(project, "-").Do() // "-" == all zones
	ctx := context.Background()

	if err != nil {
		return fmt.Errorf("failed to list clusters: %v", err)
	}

	// clusters -> nodepools -> instancegroupmanager + instancegroup -> namedports
	for _, v := range list.Clusters {
		//fmt.Printf("Cluster %q (%s) master_version: v%s zone: %s\n", v.Name, v.Status, v.CurrentMasterVersion, v.Zone)

		newNamedPort := &compute.NamedPort{
			Name: "footest1111",
			Port: 1111,
		}

		poolList, err := svc.Projects.Zones.Clusters.NodePools.List(project, v.Zone, v.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to list node pools for cluster %q: %v", v.Name, err)
		}
		for _, np := range poolList.NodePools {
			//fmt.Printf("  -> Pool %q (%s) machineType=%s node_version=v%s autoscaling=%v", np.Name, np.Status,
			//	np.Config.MachineType, np.Version, np.Autoscaling != nil && np.Autoscaling.Enabled)

			for _, ig := range np.InstanceGroupUrls {
				elm := strings.Split(ig, "/")
				npZone := elm[8]
				npIgM := elm[10]

				req, err := csvc.InstanceGroupManagers.Get(project, npZone, npIgM).Do()
				if err != nil {
					log.Fatal(err)
				}

				instanceGroup := strings.Split(req.InstanceGroup, "/")[10]
				fmt.Printf("\ninstancegroup and ports = %s -> %q -> %s\n", req.Name, req.NamedPorts, instanceGroup)

				namedPorts := append(req.NamedPorts, newNamedPort)
				rb := &compute.InstanceGroupsSetNamedPortsRequest{NamedPorts: namedPorts}
				resp, err := csvc.InstanceGroups.SetNamedPorts(project, npZone, instanceGroup, rb).Context(ctx).Do()
				if err != nil {
					//log.Fatal(err)
				}
				fmt.Printf("%#v\n", resp.HTTPStatusCode)

			}
		}
		//fmt.Printf("\n")
	}

	/*
		fmt.Println("---------------- intance groups managers -------------------")
		r, err := csvc.InstanceGroupManagers.List(project, "europe-west1-b").Do()
		fmt.Printf("%#v\n", r)

		fmt.Println("---------------- intance groups named ports -------------------")
		req, err := csvc.InstanceGroups.List(project, "europe-west1-b").Do() // XXX zone

		if err != nil {
			return fmt.Errorf("failed to instanceGroups: %v", err)
		}

		for _, i := range req.Items {
			for _, port := range i.NamedPorts {
				fmt.Printf("namedport: %s -> %d\n", port.Name, port.Port)
			}
		}
	*/

	return nil
}

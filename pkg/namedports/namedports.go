package namedports

import (
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	container "google.golang.org/api/container/v1"
)

// PortList is a group of named ports (port name, port number)
type PortList map[string]int64

// NamedPort maintains instance groups named ports in sync with a provided PortList
type NamedPort struct {
	zone    string
	cluster string
	project string
	context context.Context
	logger  *logrus.Logger
	dryrun  bool
}

type igInfo struct {
	name  string
	zone  string
	ports PortList
}

// NewNamedPort returns a NamedPort instance
func NewNamedPort(zone, cluster, project string, dryrun bool, logger *logrus.Logger) *NamedPort {
	var err error
	ctx := context.Background()

	if cluster == "" {
		log.Fatal("Cluster name is mandatory")
	}

	if project == "" {
		project, err = metadata.ProjectID()
		if err != nil {
			log.Fatalf("Could not find current GCP project: %v", err)
		}
	}

	if zone == "" {
		svc, _, err := getServices(ctx)
		if err != nil {
			log.Fatal(err)
		}

		zone, err = getClusterZone(project, cluster, svc)
		if err != nil {
			log.Fatalf("Could not find cluster zone: %v", err)
		}
	}

	return &NamedPort{
		zone:    zone,
		project: project,
		cluster: cluster,
		context: ctx,
		dryrun:  dryrun,
		logger:  logger,
	}
}

func getServices(ctx context.Context) (*container.Service, *compute.Service, error) {
	// We'll use the current host ServiceAccount if possible. If not available,
	// pass auth according to https://cloud.google.com/docs/authentication/
	// (ie. via GOOGLE_APPLICATION_CREDENTIALS environment or otherwise).
	hc, err := google.DefaultClient(ctx, container.CloudPlatformScope, compute.CloudPlatformScope)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get authenticated client: %v", err)
	}

	svc, err := container.New(hc)
	if err != nil {
		return nil, nil, fmt.Errorf("could not initialize gke client: %v", err)
	}

	csvc, err := compute.New(hc)
	if err != nil {
		return nil, nil, fmt.Errorf("could not initialize compute client: %v", err)
	}

	return svc, csvc, nil
}

// ResyncNamedPorts ensure the GKE cluster's instance groups have the
// named ports described by the provided PortList.
func (n *NamedPort) ResyncNamedPorts(expected PortList) error {
	svc, csvc, err := getServices(n.context)
	if err != nil {
		return fmt.Errorf("failed to init GCP service: %v", err)
	}

	igz, err := n.getInstanceGroups(svc, csvc)
	if err != nil {
		return fmt.Errorf("could not find cluster's instancegroups: %v", err)
	}

	for _, ig := range *igz {
		dirty := false
		for ename, eport := range expected {
			if igport, ok := ig.ports[ename]; ok {
				if eport == igport {
					continue
				}
			}
			n.logger.Infof("Need to add %s->%d port on InstanceGroup %s", ename, eport, ig.name)
			dirty = true
		}

		if !dirty {
			continue
		}

		if n.dryrun {
			fmt.Printf("Instance group %s needs a named ports update (dry-run)", ig.name)
			continue
		}

		err := n.updateNamedPorts(expected, &ig, csvc)
		if err != nil {
			return fmt.Errorf("failed to update instance group: %v", err)
		}
	}

	return nil
}

func getClusterZone(project string, cluster string, svc *container.Service) (string, error) {
	var zone string

	list, err := svc.Projects.Zones.Clusters.List(project, "-").Do() // "-" == all zones

	if err != nil {
		return zone, fmt.Errorf("failed to list clusters: %v", err)
	}

	for _, v := range list.Clusters {
		if v.Name == cluster {
			zone = v.Zone
			break
		}
	}

	return zone, nil
}

func (n *NamedPort) getInstanceGroups(svc *container.Service, csvc *compute.Service) (*[]igInfo, error) {
	var igz []igInfo

	parent := "projects/" + n.project + "/locations/" + n.zone + "/clusters/" + n.cluster
	poolList, err := svc.Projects.Locations.Clusters.NodePools.List(parent).Do()
	if err != nil {
		return &igz, fmt.Errorf("failed to list node pools for cluster %q: %v", n.cluster, err)
	}

	for _, np := range poolList.NodePools {
		for _, ig := range np.InstanceGroupUrls {
			elm := strings.Split(ig, "/")

			igroup := igInfo{
				name:  elm[10],
				zone:  elm[8],
				ports: make(PortList),
			}

			req, err := csvc.InstanceGroupManagers.Get(n.project, igroup.zone, igroup.name).Do()
			if err != nil {
				return &igz, fmt.Errorf("failed to collect named ports: %v", err)
			}
			for _, port := range req.NamedPorts {
				igroup.ports[port.Name] = port.Port
			}

			igz = append(igz, igroup)
		}
	}

	return &igz, nil
}

func (n *NamedPort) updateNamedPorts(ports PortList, ig *igInfo, csvc *compute.Service) error {
	var namedPorts []*compute.NamedPort
	mergedPorts := make(PortList)

	// we keep all the old named ports, even if not specified. hence the merge.
	for k, v := range ig.ports {
		mergedPorts[k] = v
	}
	for k, v := range ports {
		mergedPorts[k] = v
	}

	for k, v := range mergedPorts {
		namedPorts = append(namedPorts, &compute.NamedPort{Name: k, Port: v})

	}

	n.logger.Infof("Will update namedports for %s instancegroup\n", ig.name)

	rb := &compute.InstanceGroupsSetNamedPortsRequest{NamedPorts: namedPorts}
	_, err := csvc.InstanceGroups.SetNamedPorts(n.project, ig.zone, ig.name, rb).Do()

	return err
}

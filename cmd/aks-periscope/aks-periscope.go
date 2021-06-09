package main

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/Azure/aks-periscope/pkg/collector"
	"github.com/Azure/aks-periscope/pkg/diagnoser"
	"github.com/Azure/aks-periscope/pkg/exporter"
	"github.com/Azure/aks-periscope/pkg/interfaces"
	"github.com/Azure/aks-periscope/pkg/utils"
)

const (
	connectedCluster = "connectedCluster"
)

func main() {
	zipAndExportMode := true
	//exporter := &exporter.LocalMachineExporter{}
	exporters := []interfaces.Exporter{}

	var waitgroup sync.WaitGroup

	err := utils.CreateCRD()
	if err != nil {
		log.Printf("Failed to create CRD: %+v", err)
	}

	filesToZip := []string{}
	clusterType := os.Getenv("CLUSTER_TYPE")
	isConnectedCluster := strings.EqualFold(clusterType, connectedCluster)
	storageAccountName := os.Getenv("AZURE_BLOB_ACCOUNT_NAME")
	sasTokenName := os.Getenv("AZURE_BLOB_SAS_KEY")

	if isConnectedCluster && (len(storageAccountName) == 0 || len(sasTokenName) == 0) {
		exporters = append(exporters, &exporter.LocalMachineExporter{})
	} else {
		exporters = append(exporters, &exporter.AzureBlobExporter{})
	}

	exporter := exporters[0]

	collectors := []interfaces.Collector{}
	containerLogsCollector := collector.NewContainerLogsCollector(exporter)
	networkOutboundCollector := collector.NewNetworkOutboundCollector(5, exporter)
	dnsCollector := collector.NewDNSCollector(exporter)
	kubeObjectsCollector := collector.NewKubeObjectsCollector(exporter)
	systemLogsCollector := collector.NewSystemLogsCollector(exporter)
	ipTablesCollector := collector.NewIPTablesCollector(exporter)
	nodeLogsCollector := collector.NewNodeLogsCollector(exporter)
	kubeletCmdCollector := collector.NewKubeletCmdCollector(exporter)
	systemPerfCollector := collector.NewSystemPerfCollector(exporter)
	helmCollector := collector.NewHelmCollector(exporter)

	if isConnectedCluster {
		collectors = append(collectors, containerLogsCollector)
		collectors = append(collectors, dnsCollector)
		collectors = append(collectors, helmCollector)
		collectors = append(collectors, kubeObjectsCollector)
		collectors = append(collectors, networkOutboundCollector)

	} else {
		collectors = append(collectors, containerLogsCollector)
		collectors = append(collectors, dnsCollector)
		collectors = append(collectors, kubeObjectsCollector)
		collectors = append(collectors, networkOutboundCollector)
		collectors = append(collectors, systemLogsCollector)
		collectors = append(collectors, ipTablesCollector)
		collectors = append(collectors, nodeLogsCollector)
		collectors = append(collectors, kubeletCmdCollector)
		collectors = append(collectors, systemPerfCollector)
	}

	for _, c := range collectors {
		waitgroup.Add(1)
		go func(c interfaces.Collector) {
			log.Printf("Collector: %s, collect data\n", c.GetName())
			err := c.Collect()
			if err != nil {
				log.Printf("Collector: %s, collect data failed: %+v\n", c.GetName(), err)
			}

			log.Printf("Collector: %s, export data\n", c.GetName())
			err = c.Export()
			if err != nil {
				log.Printf("Collector: %s, export data failed: %+v\n", c.GetName(), err)
			}

			if isConnectedCluster {
				for _, file := range c.GetFiles() {
					filesToZip = append(filesToZip, file)
					log.Printf("Collector: %s, added to zip file list", c.GetName())
				}
			}

			waitgroup.Done()
		}(c)
	}

	waitgroup.Wait()

	diagnosers := []interfaces.Diagnoser{}
	diagnosers = append(diagnosers, diagnoser.NewNetworkConfigDiagnoser(dnsCollector, kubeletCmdCollector, exporter))
	diagnosers = append(diagnosers, diagnoser.NewNetworkOutboundDiagnoser(networkOutboundCollector, exporter))

	for _, d := range diagnosers {
		waitgroup.Add(1)
		go func(d interfaces.Diagnoser) {
			log.Printf("Diagnoser: %s, diagnose data\n", d.GetName())
			err := d.Diagnose()
			if err != nil {
				log.Printf("Diagnoser: %s, diagnose data failed: %+v\n", d.GetName(), err)
			}

			log.Printf("Diagnoser: %s, export data\n", d.GetName())
			err = d.Export()
			if err != nil {
				log.Printf("Diagnoser: %s, export data failed: %+v\n", d.GetName(), err)
			}

			if isConnectedCluster {
				for _, file := range d.GetFiles() {
					filesToZip = append(filesToZip, file)
					log.Printf("Diagnoser: %s, added to zip file list", d.GetName())
				}
			}

			waitgroup.Done()
		}(d)
	}

	waitgroup.Wait()

	if zipAndExportMode {
		log.Print("Zip and export result files")
		err := zipAndExport(exporter, filesToZip)
		if err != nil {
			log.Printf("Failed to zip and export result files: %+v", err)
		}
	}

	select {}
}

// zipAndExport zip the results and export
func zipAndExport(exporter interfaces.Exporter, filesToZip []string) error {
	hostName, err := utils.GetHostName()
	if err != nil {
		log.Printf("hostname issue")
		return err
	}

	creationTimeStamp, err := utils.GetCreationTimeStamp()
	if err != nil {
		log.Printf("timestamp issue")
		return err
	}

	sourcePathOnHost := "/var/log/aks-periscope/" + strings.Replace(creationTimeStamp, ":", "-", -1) + "/" + hostName
	zipFileOnHost := sourcePathOnHost + "/" + hostName + ".zip"
	zipFileOnContainer := strings.TrimPrefix(zipFileOnHost, "/var/log")
	clusterType := os.Getenv("CLUSTER_TYPE")
	isConnectedCluster := strings.EqualFold(clusterType, connectedCluster)

	if !isConnectedCluster {
		_, err = utils.RunCommandOnHost("zip", "-r", zipFileOnHost, sourcePathOnHost)
		if err != nil {
			return err
		}

		err = exporter.Export([]string{zipFileOnContainer})
		if err != nil {
			log.Printf("export issue")
			return err
		}
	} else {
		//output, err := utils.RunCommandOnHost("ls", sourcePathOnHost)
		//if err != nil {
		//	return err
		//}
		//filesToZip := strings.Split(output, "\n")
		log.Printf("files on host: %s", filesToZip)
		err = exporter.Export(filesToZip)
		if err != nil {
			log.Printf("export issue")
			return err
		}
	}

	return nil
}

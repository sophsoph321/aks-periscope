package exporter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/aks-periscope/pkg/interfaces"
	"github.com/Azure/aks-periscope/pkg/utils"
)

// LocalMachineExporter defines an Local Machine Exporter
type LocalMachineExporter struct{}

var _ interfaces.Exporter = &LocalMachineExporter{}

// Export implements the interface method
func (exporter *LocalMachineExporter) Export(files []string) error {
	//rootPath, err := utils.CreateLocalMachineDir("periscope_logs")
	//if err != nil {
	//	return err
	//}
	for _, file := range files {
		hostName, err := utils.GetHostName()
		if err != nil {
			return err
		}

		filePath := strings.Split(file, "/")
		newDir := "periscope-logs-" + hostName
		dirname := filepath.Join(newDir, filePath[0])
		str := newDir + "/" + filePath[0]
		for _, path := range filePath[1 : len(filePath)-1] {
			dirname = filepath.Join(str, path)
			str = str + "/" + path
		}

		//check if directory already exists in container
		_, err = os.Stat(dirname)

		if os.IsNotExist(err) {
			err = os.MkdirAll(dirname, os.ModePerm)
			if err != nil {
				return fmt.Errorf("Fail to create dir %s: %+v", dirname, err)
			}
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("Fail to open file %s: %+v", file, err)
		}

		dst, err := os.Create(newDir + file)
		if err != nil {
			return fmt.Errorf("Fail to create file %s: %+v", newDir+file, err)
		}

		defer dst.Close()
		_, err = io.Copy(dst, f)
		if err != nil {
			return err
		}
	}
	return nil
}

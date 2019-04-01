package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type (
	Workspace struct {
		Size         int64     `json:"size"`
		Key          string    `json:"key"`
		LastUsed     time.Time `json:"last-used"`
		CreationTime time.Time `json:"creation-time"`
	}
	Spec struct {
		Workspaces   map[string]*Workspace `json:"workspaces"`
		CreationTime time.Time
	}

	WorkspaceSlice []*Workspace

	Stat struct {
		Total uint64
		Free  uint64
	}

	Cleaner interface {
		Remove()
	}
)

const (
	workspacesSpecFileName = "workspace.json"

	commandInit   = "init"
	commandUpdate = "update"
	commandClean  = "clean"

	cleaningStrategyFreePerecentage = "perecentage"
	cleaningStrategyNotUsedAtLeast  = "unused"
)

func main() {
	command := getEnvOrDie("COMMAND")
	workspace := getEnvOrDie("WORKSPACE")
	fmt.Printf("Running command: %s where workspace=%s\n", command, workspace)
	if command == commandInit {
		createEmptySpecOrDie(workspace)
		os.Exit(0)
	}

	if command == commandUpdate {
		spec := loadSpecOrDie(workspace)
		key := getEnvOrDie("KEY")
		size := calculateDirecotySizeOrDie(workspace + "/" + key)
		fmt.Printf("Directory %s, Size %d\n", workspace, size)
		if s, ok := spec.Workspaces[key]; !ok {
			fmt.Printf("Key %s not exist, creating\n", key)
			spec.Workspaces[key] = &Workspace{
				Key:          key,
				Size:         size,
				LastUsed:     time.Now(),
				CreationTime: time.Now(),
			}
		} else {
			fmt.Printf("Updating key %s\n", key)
			s.LastUsed = time.Now()
			s.Size = size
		}
		updateSpecOrDie(workspace, spec)

		os.Exit(0)
	}

	if command == commandClean {
		spec := loadSpecOrDie(workspace)
		strategy := getEnvOrDie("CLEAN_STRATEGY")
		strategies := strings.Split(strategy, ":")
		for _, s := range strategies {
			fmt.Printf("Cleaning workspaces with strategy %s\n", s)
			if s == cleaningStrategyFreePerecentage {
				cleanPercentageStrategy(workspace, spec)
			}

			if s == cleaningStrategyNotUsedAtLeast {
				cleanUnusedStrategy(workspace, spec)
			}

		}
	}
}

func cleanPercentageStrategy(path string, spec *Spec) {
	perecentage, err := strconv.Atoi(getEnvOrDie("PERCENTAGE_TO_KEEP_AVAILABLE"))
	if err != nil {
		dieOnError(fmt.Errorf("Failed to convert PERCENTAGE_TO_KEEP_AVAILABLE to int, original error: %s", err.Error()))
	}
	fmt.Printf("Keeping at least %d%s available \n", perecentage, "%")
	sorted := sortWorkspacesByTime(spec.Workspaces)
	s := buidStatOrDie(path)
	available := percentageChange(s.Free, s.Total)
	if perecentage > available {
		fmt.Printf("%d > %d, starting to clean\n", perecentage, available)
		for _, s := range sorted {
			fmt.Printf("Checking key: %s, size: %d, last-used: %s\n", s.Key, s.Size, s.LastUsed.Format(time.UnixDate))
			stat := buidStatOrDie(path)
			available := percentageChange(stat.Free, stat.Total)
			fmt.Printf("Available memory: %d%s\n", available, "%")
			if perecentage > available {
				fmt.Printf("Criteria matched, cleaning workspace %s\n", s.Key)
				cleanWorkspace(path, s)
				delete(spec.Workspaces, s.Key)
			}
			fmt.Println()
			fmt.Println()
		}
		updateSpecOrDie(path, spec)
	}
}

func cleanUnusedStrategy(path string, spec *Spec) {
	now := time.Now()
	days, err := strconv.Atoi(getEnvOrDie("UNUSED_N_DAYS"))
	if err != nil {
		dieOnError(fmt.Errorf("Failed to convert UNUSED_N_DAYS to int, original error: %s", err.Error()))
	}
	fmt.Printf("Cleaning workspaces that wasnt used for the last %d days\n", days)
	sorted := sortWorkspacesByTime(spec.Workspaces)
	for _, s := range sorted {
		if s.LastUsed.Add(time.Hour * 24 * time.Duration(days)).Before(now) {
			fmt.Printf("Workspace was used last time at %s and not been used for the last %s days, deleting\n", s.Key, s.LastUsed.Format(time.UnixDate))
			cleanWorkspace(path, s)
			delete(spec.Workspaces, s.Key)
		}
	}
	updateSpecOrDie(path, spec)
}

func cleanWorkspace(path string, ws *Workspace) {
	err := os.RemoveAll(path + "/" + ws.Key)
	if err != nil {
		fmt.Printf("Failed to delete %s\n", err.Error())
	}
}

func percentageChange(old, new uint64) int {
	diff := float64(new - old)
	delta := (diff / float64(old)) * 100
	return int(100 - math.Round(delta))
}

func buidStatOrDie(workspace string) *Stat {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(workspace, &fs)
	if err != nil {
		fmt.Printf("Failed to create stats with error: %s\n", err.Error())
		os.Exit(1)
	}
	stat := &Stat{
		Total: fs.Blocks * uint64(fs.Bsize),
		Free:  fs.Bfree * uint64(fs.Bsize),
	}
	fmt.Printf("Total available %d\n", stat.Total)
	fmt.Printf("Total free %d\n", stat.Free)
	return stat

}

func (p WorkspaceSlice) Len() int {
	return len(p)
}

func (p WorkspaceSlice) Less(i, j int) bool {
	return p[i].LastUsed.Before(p[j].LastUsed)
}

func (p WorkspaceSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func sortWorkspacesByTime(workspaces map[string]*Workspace) WorkspaceSlice {
	slice := make(WorkspaceSlice, 0, len(workspaces))
	for _, ws := range workspaces {
		slice = append(slice, ws)
	}

	sort.Sort(slice)
	return slice
}

func loadSpecOrDie(path string) *Spec {
	f, err := ioutil.ReadFile(path + "/" + "workspace.json")
	if err != nil {
		fmt.Printf("Failed to load spec file with error: %s\n", err.Error())
		os.Exit(1)
	}

	spec := &Spec{}

	if err := json.Unmarshal(f, spec); err != nil {
		fmt.Printf("Failed to load spec file with error: %s\n", err.Error())
		os.Exit(1)
	}
	return spec
}

func createEmptySpecOrDie(path string) {
	spec := &Spec{
		CreationTime: time.Now(),
		Workspaces:   make(map[string]*Workspace, 0),
	}
	updateSpecOrDie(path, spec)

}

func calculateDirecotySizeOrDie(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		fmt.Printf("Failed to calculate size of directory with error: %s\n", err.Error())
		os.Exit(1)
	}
	return size
}

func updateSpecOrDie(path string, spec *Spec) {
	result, err := json.Marshal(spec)
	fmt.Printf(string(result))
	if err != nil {
		fmt.Printf("Failed to marshel spec into bytes with error: %s\n", err.Error())
		os.Exit(1)
	}
	if err := ioutil.WriteFile(path+"/"+workspacesSpecFileName, result, os.ModePerm); err != nil {
		fmt.Printf("Failed to write spec to file with error: %s\n", err.Error())
		os.Exit(1)
	}
}

func getEnvOrDie(name string) string {
	if v := os.Getenv(name); v == "" {
		dieOnError(fmt.Errorf("%s environment variable is not set, exiting", name))
		return ""
	} else {
		return v
	}
}

func dieOnError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

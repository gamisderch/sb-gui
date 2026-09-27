package process

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ProcessNode represents a process in the tree
type ProcessNode struct {
	PID           int
	PPID          int
	Name          string
	CleanName     string // Cleaned name (e.g. cyberpunk2077.exe)
	User          string
	UID           int
	IsSystem      bool
	IsWine        bool
	Sockets       int
	DownRate      string
	UpRate        string
	Children      []*ProcessNode
	IsBypass      bool   // true = in Right column (Direct / Bypass); false = in Left column (VPN)
	IsCollapsed   bool   // true if children are collapsed (E key)
	ChildCount    int
	AllBranchPIDs []int  // All PIDs in this sub-tree
}

// ProcessManager manages the process tree and bypass list
type ProcessManager struct {
	mu          sync.RWMutex
	BypassNames map[string]bool // Persistent bypass rules by process name
	BypassPIDs  map[int]bool    // Active bypass by PID
	HideSystem  bool            // S key: hide system processes
	Collapse    bool            // E key: collapse identical worker threads/subprocesses
	cachedUsers map[int]string
}

// NewProcessManager creates a new ProcessManager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		BypassNames: make(map[string]bool),
		BypassPIDs:  make(map[int]bool),
		cachedUsers: make(map[int]string),
		HideSystem:  false,
		Collapse:    false,
	}
}

// ToggleBypass switches a single process or whole tree between VPN and Direct (Bypass)
func (pm *ProcessManager) ToggleBypass(node *ProcessNode, wholeTree bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	targetBypass := !node.IsBypass

	if wholeTree {
		pidsToToggle := node.AllBranchPIDs
		if len(pidsToToggle) == 0 {
			pidsToToggle = []int{node.PID}
		}
		for _, pid := range pidsToToggle {
			if targetBypass {
				pm.BypassPIDs[pid] = true
			} else {
				delete(pm.BypassPIDs, pid)
			}
		}
		namesToToggle := collectSubtreeProcessNames(node)
		for _, name := range namesToToggle {
			if targetBypass {
				pm.BypassNames[name] = true
			} else {
				delete(pm.BypassNames, name)
			}
		}
	} else {
		if targetBypass {
			pm.BypassPIDs[node.PID] = true
			pm.BypassNames[node.CleanName] = true
			if node.Name != "" && node.Name != node.CleanName {
				pm.BypassNames[node.Name] = true
			}
		} else {
			delete(pm.BypassPIDs, node.PID)
			delete(pm.BypassNames, node.CleanName)
			if node.Name != "" {
				delete(pm.BypassNames, node.Name)
			}
		}
	}
}

func collectSubtreeProcessNames(node *ProcessNode) []string {
	var names []string
	addName := func(n string) {
		trimmed := strings.TrimSpace(n)
		if trimmed != "" {
			names = append(names, trimmed)
		}
	}
	addName(node.CleanName)
	if node.Name != "" && node.Name != node.CleanName {
		addName(node.Name)
	}
	for _, ch := range node.Children {
		names = append(names, collectSubtreeProcessNames(ch)...)
	}
	return names
}

// GetBypassNamesList returns a comprehensive list of process names, comms, and Wine loaders for sing-box rules
func (pm *ProcessManager) GetBypassNamesList() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	seen := make(map[string]bool)
	hasWineProcess := false

	for name := range pm.BypassNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = true

		lower := strings.ToLower(trimmed)
		if strings.HasSuffix(lower, ".exe") {
			hasWineProcess = true
			baseWithoutExe := strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
			if baseWithoutExe != "" {
				seen[baseWithoutExe] = true
				if len(baseWithoutExe) > 15 {
					seen[baseWithoutExe[:15]] = true
				}
			}
			if len(trimmed) > 15 {
				seen[trimmed[:15]] = true
			}
		}
		if strings.Contains(lower, "wine") || strings.Contains(lower, "preloader") || strings.Contains(lower, "proton") || strings.Contains(lower, "fsd") {
			hasWineProcess = true
		}
		if strings.Contains(lower, "fsd") {
			seen["FSD"] = true
			seen["FSD.exe"] = true
			seen["FSD-Win64-Shipping.exe"] = true
			seen["FSD-Win64-Shipping"] = true
			seen["FSD-Win64-Shipp"] = true
		}
		if strings.Contains(lower, "steam") {
			seen["steamwebhelper"] = true
			seen["steam"] = true
			seen["steam_osx"] = true
			seen["steam_linux"] = true
		}
		if len(trimmed) > 15 {
			seen[trimmed[:15]] = true
		}
	}

	// If any Windows / Wine / Proton process is bypassed, ensure Wine and Proton socket wrappers are bypassed
	if hasWineProcess {
		seen["wine64-preloader"] = true
		seen["wine-preloader"] = true
		seen["wineserver"] = true
		seen["wine"] = true
		seen["winedevice.exe"] = true
		seen["winedevice"] = true
		seen["pv-adverb"] = true
		seen["pressure-vessel-adverb"] = true
		seen["srt-bwrap"] = true
		seen["bwrap"] = true
		seen["steamwebhelper"] = true
		seen["steam"] = true
		seen["reaper"] = true
		seen["gameoverlayui"] = true
		seen["xalia.exe"] = true
		seen["xalia"] = true
		seen["proton"] = true
	}

	var list []string
	for n := range seen {
		list = append(list, n)
	}
	sort.Strings(list)
	return list
}

// Refresh scans /proc and returns two stably sorted trees: (vpnTree, bypassTree)
func (pm *ProcessManager) Refresh() ([]*ProcessNode, []*ProcessNode, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	nodesByPID := make(map[int]*ProcessNode)
	var allNodes []*ProcessNode

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		node := pm.readProcessInfo(pid)
		if node == nil {
			continue
		}

		if pm.BypassNames[node.CleanName] || pm.BypassNames[node.Name] || pm.BypassPIDs[node.PID] {
			node.IsBypass = true
		} else {
			node.IsBypass = false
		}

		nodesByPID[pid] = node
		allNodes = append(allNodes, node)
	}

	// Helper to build a clean, deterministically sorted tree
	buildTreeForSubset := func(forBypass bool) []*ProcessNode {
		subset := make(map[int]*ProcessNode)
		var pids []int
		for pid, node := range nodesByPID {
			if node.IsBypass == forBypass {
				cloned := *node
				cloned.Children = nil
				subset[pid] = &cloned
				pids = append(pids, pid)
			}
		}

		// STABLE SORT: sort by PID ascending so order NEVER randomizes!
		sort.Ints(pids)

		var roots []*ProcessNode
		for _, pid := range pids {
			node := subset[pid]
			parent, parentInSubset := subset[node.PPID]
			if parentInSubset && node.PPID > 0 && node.PPID != pid {
				parent.Children = append(parent.Children, node)
			} else {
				roots = append(roots, node)
			}
		}

		// Stable sort children of every node by PID
		for _, node := range subset {
			if len(node.Children) > 1 {
				sort.Slice(node.Children, func(i, j int) bool {
					return node.Children[i].PID < node.Children[j].PID
				})
			}
			node.AllBranchPIDs = collectBranchPIDs(node)
			node.ChildCount = 0
		}

		// Sort roots by PID
		sort.Slice(roots, func(i, j int) bool {
			return roots[i].PID < roots[j].PID
		})

		return pm.filterTree(roots)
	}

	vpnRoots := buildTreeForSubset(false)
	bypassRoots := buildTreeForSubset(true)

	// Helper to check if a process name is an internal Wine helper/socket wrapper
	isInternalWineLoader := func(lower string) bool {
		switch lower {
		case "wine64-preloader", "wine-preloader", "wineserver", "winedevice.exe", "pv-adverb", "srt-bwrap":
			return true
		default:
			return false
		}
	}

	// Collect active names in bypassRoots
	activeBypassNames := make(map[string]bool)
	var collectActiveNames func(n *ProcessNode)
	collectActiveNames = func(n *ProcessNode) {
		activeBypassNames[strings.ToLower(n.CleanName)] = true
		if n.Name != "" {
			activeBypassNames[strings.ToLower(n.Name)] = true
		}
		for _, ch := range n.Children {
			collectActiveNames(ch)
		}
	}
	for _, root := range bypassRoots {
		collectActiveNames(root)
	}

	// For any name saved in pm.BypassNames that has no active running process in /proc,
	// add an inactive placeholder root so it remains visible in the Bypass column
	var offlineRoots []*ProcessNode
	seenOffline := make(map[string]bool)
	syntheticPID := -1
	for name := range pm.BypassNames {
		clean := strings.TrimSpace(name)
		if clean == "" {
			continue
		}
		lower := strings.ToLower(clean)
		if isInternalWineLoader(lower) {
			continue
		}
		if activeBypassNames[lower] {
			continue
		}
		// If both "game.exe" and "game" exist in BypassNames, prefer "game.exe"
		if !strings.HasSuffix(lower, ".exe") && pm.BypassNames[clean+".exe"] {
			continue
		}
		if seenOffline[lower] {
			continue
		}
		seenOffline[lower] = true

		offlineRoots = append(offlineRoots, &ProcessNode{
			PID:           syntheticPID,
			PPID:          0,
			Name:          clean,
			CleanName:     clean,
			User:          "-",
			UID:           -1,
			IsSystem:      false,
			IsWine:        strings.HasSuffix(lower, ".exe"),
			Sockets:       0,
			DownRate:      "0 B/s",
			UpRate:        "0 B/s",
			IsBypass:      true,
			IsCollapsed:   false,
			ChildCount:    0,
			AllBranchPIDs: []int{syntheticPID},
		})
		syntheticPID--
	}
	sort.Slice(offlineRoots, func(i, j int) bool {
		return strings.ToLower(offlineRoots[i].CleanName) < strings.ToLower(offlineRoots[j].CleanName)
	})
	bypassRoots = append(bypassRoots, offlineRoots...)

	return vpnRoots, bypassRoots, nil
}

func collectBranchPIDs(node *ProcessNode) []int {
	pids := []int{node.PID}
	for _, ch := range node.Children {
		pids = append(pids, collectBranchPIDs(ch)...)
	}
	return pids
}

func (pm *ProcessManager) filterTree(roots []*ProcessNode) []*ProcessNode {
	var visibleRoots []*ProcessNode
	for _, root := range roots {
		if pm.HideSystem && root.IsSystem {
			continue
		}
		visibleRoots = append(visibleRoots, root)
	}

	if !pm.Collapse {
		var resetNode func(n *ProcessNode)
		resetNode = func(n *ProcessNode) {
			n.IsCollapsed = false
			n.ChildCount = 0
			for _, ch := range n.Children {
				resetNode(ch)
			}
		}
		for _, r := range visibleRoots {
			resetNode(r)
		}
		return visibleRoots
	}

	// Collapse identical copies when pm.Collapse is true
	groupedRoots := make(map[string][]*ProcessNode)
	var rootKeys []string
	for _, r := range visibleRoots {
		key := strings.ToLower(r.CleanName)
		if len(groupedRoots[key]) == 0 {
			rootKeys = append(rootKeys, key)
		}
		groupedRoots[key] = append(groupedRoots[key], r)
	}

	var collapsedRoots []*ProcessNode
	for _, key := range rootKeys {
		group := groupedRoots[key]
		head := group[0]
		if len(group) > 1 {
			totalCount := 0
			extraSockets := 0
			var extraChildren []*ProcessNode
			for _, r := range group {
				totalCount++
				if r != head {
					extraSockets += r.Sockets
				}
				for _, ch := range r.Children {
					if strings.EqualFold(ch.CleanName, head.CleanName) {
						totalCount++
						extraSockets += ch.Sockets
					} else {
						extraChildren = append(extraChildren, ch)
					}
				}
			}
			head.ChildCount = totalCount
			head.IsCollapsed = true
			head.Sockets += extraSockets
			head.Children = append(head.Children, extraChildren...)
			collapseSubtree(head)
			collapsedRoots = append(collapsedRoots, head)
		} else {
			collapseSubtree(head)
			collapsedRoots = append(collapsedRoots, head)
		}
	}

	return collapsedRoots
}

func collapseSubtree(node *ProcessNode) {
	if node == nil {
		return
	}

	// 1. Absorb all descendants with the exact same CleanName as this node
	sameNameCount := 1
	absorbedSockets := 0
	var differentChildren []*ProcessNode

	var collectDescendants func(curr *ProcessNode)
	collectDescendants = func(curr *ProcessNode) {
		for _, ch := range curr.Children {
			if strings.EqualFold(ch.CleanName, node.CleanName) {
				sameNameCount++
				absorbedSockets += ch.Sockets
				collectDescendants(ch)
			} else {
				differentChildren = append(differentChildren, ch)
			}
		}
	}
	collectDescendants(node)

	if sameNameCount > 1 {
		node.ChildCount = sameNameCount
		node.IsCollapsed = true
		node.Sockets += absorbedSockets
	} else {
		node.ChildCount = 0
		node.IsCollapsed = false
	}

	// 2. Group the remaining children by CleanName
	grouped := make(map[string][]*ProcessNode)
	var groupOrder []string

	for _, ch := range differentChildren {
		key := strings.ToLower(ch.CleanName)
		if len(grouped[key]) == 0 {
			groupOrder = append(groupOrder, key)
		}
		grouped[key] = append(grouped[key], ch)
	}

	var newChildren []*ProcessNode
	for _, key := range groupOrder {
		group := grouped[key]
		head := group[0]

		if len(group) > 1 {
			totalSibCount := 0
			extraSockets := 0
			var extraChildren []*ProcessNode

			for _, sib := range group {
				totalSibCount++
				if sib != head {
					extraSockets += sib.Sockets
				}
				for _, sch := range sib.Children {
					if strings.EqualFold(sch.CleanName, head.CleanName) {
						totalSibCount++
						extraSockets += sch.Sockets
					} else {
						extraChildren = append(extraChildren, sch)
					}
				}
			}

			head.ChildCount = totalSibCount
			head.IsCollapsed = true
			head.Sockets += extraSockets
			head.Children = extraChildren
			collapseSubtree(head)
			newChildren = append(newChildren, head)
		} else {
			collapseSubtree(head)
			newChildren = append(newChildren, head)
		}
	}

	node.Children = newChildren
}

func (pm *ProcessManager) readProcessInfo(pid int) *ProcessNode {
	procPath := fmt.Sprintf("/proc/%d", pid)

	commBytes, err := os.ReadFile(filepath.Join(procPath, "comm"))
	if err != nil {
		return nil
	}
	comm := strings.TrimSpace(string(commBytes))

	statBytes, err := os.ReadFile(filepath.Join(procPath, "stat"))
	if err != nil {
		return nil
	}
	ppid := parsePPIDFromStat(string(statBytes))

	cleanName := pm.extractCleanName(procPath, comm)

	uid := pm.readUID(procPath)
	username := pm.getUsername(uid)

	isSystem := uid == 0 || isKnownSystemDaemon(comm)
	sockets := countSockets(procPath)
	lowerClean := strings.ToLower(cleanName)
	lowerComm := strings.ToLower(comm)
	isWine := strings.HasSuffix(lowerClean, ".exe") || strings.Contains(lowerComm, "wine") || strings.Contains(lowerComm, "proton")

	return &ProcessNode{
		PID:       pid,
		PPID:      ppid,
		Name:      comm,
		CleanName: cleanName,
		User:      username,
		UID:       uid,
		IsSystem:  isSystem,
		IsWine:    isWine,
		Sockets:   sockets,
		DownRate:  "0 B/s",
		UpRate:    "0 B/s",
	}
}


func (pm *ProcessManager) extractCleanName(procPath, comm string) string {
	cmdlineBytes, err := os.ReadFile(filepath.Join(procPath, "cmdline"))
	if err != nil || len(cmdlineBytes) == 0 {
		return comm
	}

	args := bytes.Split(cmdlineBytes, []byte{0})

	lowerComm := strings.ToLower(comm)
	if strings.Contains(lowerComm, "wine") || strings.Contains(lowerComm, "preloader") {
		for _, arg := range args {
			argStr := string(arg)
			lowerArg := strings.ToLower(argStr)
			if strings.HasSuffix(lowerArg, ".exe") {
				base := filepath.Base(strings.ReplaceAll(argStr, `\`, `/`))
				if base != "" && !strings.EqualFold(base, "services.exe") && !strings.EqualFold(base, "explorer.exe") {
					return base
				}
			}
		}
	}

	for _, arg := range args {
		argStr := string(arg)
		lower := strings.ToLower(argStr)
		if strings.HasSuffix(lower, ".exe") {
			base := filepath.Base(strings.ReplaceAll(argStr, `\`, `/`))
			if base != "" {
				return base
			}
		}
	}

	if len(args) > 0 && len(args[0]) > 0 {
		binBase := filepath.Base(string(args[0]))
		if binBase != "" && len(binBase) < 30 && !strings.HasPrefix(binBase, "python") && !strings.HasPrefix(binBase, "sh") {
			return binBase
		}
	}

	return comm
}

func parsePPIDFromStat(stat string) int {
	closeParen := strings.LastIndex(stat, ")")
	if closeParen == -1 || closeParen+2 >= len(stat) {
		return 0
	}
	fields := strings.Fields(stat[closeParen+2:])
	if len(fields) >= 2 {
		ppid, _ := strconv.Atoi(fields[1])
		return ppid
	}
	return 0
}

func (pm *ProcessManager) readUID(procPath string) int {
	statusBytes, err := os.ReadFile(filepath.Join(procPath, "status"))
	if err != nil {
		return -1
	}

	for _, line := range strings.Split(string(statusBytes), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ := strconv.Atoi(fields[1])
				return uid
			}
		}
	}
	return -1
}

func (pm *ProcessManager) getUsername(uid int) string {
	if uid < 0 {
		return "unknown"
	}
	if name, ok := pm.cachedUsers[uid]; ok {
		return name
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err == nil && u.Username != "" {
		pm.cachedUsers[uid] = u.Username
		return u.Username
	}
	if uid == 0 {
		return "root"
	}
	return strconv.Itoa(uid)
}

func isKnownSystemDaemon(comm string) bool {
	systemDaemons := map[string]bool{
		"systemd":         true,
		"systemd-journal": true,
		"systemd-logind":  true,
		"systemd-udevd":   true,
		"niri":            true,
		"sway":            true,
		"dbus-daemon":     true,
		"dbus-broker":     true,
		"polkitd":         true,
		"NetworkManager":  true,
		"accounts-daemon": true,
		"upowerd":         true,
		"udisksd":         true,
		"geoclue":         true,
		"pipewire":        true,
		"wireplumber":     true,
		"pulseaudio":      true,
		"rtkit-daemon":    true,
		"cron":            true,
		"atd":             true,
	}
	return systemDaemons[comm]
}

func countSockets(procPath string) int {
	fdPath := filepath.Join(procPath, "fd")
	dir, err := os.Open(fdPath)
	if err != nil {
		return 0
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return 0
	}

	sockets := 0
	for _, name := range names {
		linkTarget, err := os.Readlink(filepath.Join(fdPath, name))
		if err == nil && strings.HasPrefix(linkTarget, "socket:[") {
			sockets++
		}
	}
	return sockets
}

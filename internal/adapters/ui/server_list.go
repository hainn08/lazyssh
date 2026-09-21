// Copyright 2025.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Adembc/lazyssh/internal/core/domain"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ServerList struct {
	*tview.List
	servers           []domain.Server
	displayItems      []displayItem
	groupExpanded     map[string]bool
	rebuilding        bool
	onSelection       func(domain.Server)
	onSelectionChange func(domain.Server)
	onReturnToSearch  func()
}

type displayItem struct {
	isGroup bool
	group   string         // source filename (empty when isGroup=false)
	server  *domain.Server // nil when isGroup=true
}

func NewServerList() *ServerList {
	list := &ServerList{
		List:          tview.NewList(),
		groupExpanded: make(map[string]bool),
	}
	list.build()
	return list
}

func (sl *ServerList) build() {
	sl.List.ShowSecondaryText(false)
	sl.List.SetBorder(true).
		SetTitle(" Servers ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.Color238).
		SetTitleColor(tcell.Color250)
	sl.List.
		SetSelectedBackgroundColor(tcell.Color24).
		SetSelectedTextColor(tcell.Color255).
		SetHighlightFullLine(true)

	// SetChangedFunc fires on every navigation (arrow keys, SetCurrentItem).
	// Use it ONLY for updating details — never toggle groups here.
	sl.List.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if sl.rebuilding {
			return
		}
		if index >= 0 && index < len(sl.displayItems) {
			item := sl.displayItems[index]
			if !item.isGroup && item.server != nil && sl.onSelectionChange != nil {
				sl.onSelectionChange(*item.server)
			}
		}
	})

	// SetSelectedFunc fires only on explicit Enter/click — safe for toggling groups.
	sl.List.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(sl.displayItems) {
			item := sl.displayItems[index]
			if item.isGroup {
				sl.toggleGroup(item.group)
			} else if item.server != nil && sl.onSelection != nil {
				sl.onSelection(*item.server)
			}
		}
	})

	// SetInputCapture handles keyboard navigation within the list.
	// Left/Right arrows toggle group expansion (not return to search).
	sl.List.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			// Go back to parent group or collapse current group
			currentIdx := sl.List.GetCurrentItem()
			if currentIdx >= 0 && currentIdx < len(sl.displayItems) {
				item := sl.displayItems[currentIdx]
				if !item.isGroup && item.server != nil && item.server.SourceFile != domain.SourceFileMain {
					// Collapse the current server's group
					sl.toggleGroup(item.server.SourceFile)
					return nil
				}
			}
			// If at top level, return to search
			if sl.onReturnToSearch != nil {
				sl.onReturnToSearch()
			}
			return nil
		case tcell.KeyRight:
			// Expand the current group or go deeper
			currentIdx := sl.List.GetCurrentItem()
			if currentIdx >= 0 && currentIdx < len(sl.displayItems) {
				item := sl.displayItems[currentIdx]
				if item.isGroup {
					if !sl.groupExpanded[item.group] {
						sl.toggleGroup(item.group)
					}
					return nil
				}
			}
			// If on a server item and group is collapsed, expand it
			if currentIdx >= 0 && currentIdx < len(sl.displayItems) {
				item := sl.displayItems[currentIdx]
				if !item.isGroup && item.server != nil {
					sf := item.server.SourceFile
					if sf == "" {
						sf = domain.SourceFileMain
					}
					if !sl.groupExpanded[sf] {
						sl.toggleGroup(sf)
						return nil
					}
				}
			}
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyESC:
			if sl.onReturnToSearch != nil {
				sl.onReturnToSearch()
			}
			return nil
		}
		return event
	})
}

func (sl *ServerList) toggleGroup(groupName string) {
	sl.groupExpanded[groupName] = !sl.groupExpanded[groupName]
	sl.rebuildDisplayItems()
}

// onGroupToggle checks if the current selection is a group header and toggles it.
// Returns true if a group was toggled.
func (sl *ServerList) onGroupToggle() bool {
	index := sl.List.GetCurrentItem()
	if index >= 0 && index < len(sl.displayItems) {
		item := sl.displayItems[index]
		if item.isGroup {
			sl.toggleGroup(item.group)
			return true
		}
	}
	return false
}

func (sl *ServerList) rebuildDisplayItems() {
	sl.rebuilding = true
	defer func() { sl.rebuilding = false }()

	sl.displayItems = nil
	sl.List.Clear()

	// Group servers by SourceFile from the source-of-truth slice
	groups := make(map[string][]domain.Server)
	for _, server := range sl.servers {
		sourceFile := server.SourceFile
		if sourceFile == "" {
			sourceFile = domain.SourceFileMain
		}
		groups[sourceFile] = append(groups[sourceFile], server)
	}

	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for _, groupName := range groupNames {
		sl.displayItems = append(sl.displayItems, displayItem{
			isGroup: true,
			group:   groupName,
		})
		if sl.groupExpanded[groupName] {
			for _, server := range groups[groupName] {
				sl.displayItems = append(sl.displayItems, displayItem{
					isGroup: false,
					server:  &server,
				})
			}
		}
	}

	for _, item := range sl.displayItems {
		sl.addItemToList(item)
	}

	// Auto-select first server item (skip group headers)
	for i, item := range sl.displayItems {
		if !item.isGroup && item.server != nil {
			sl.List.SetCurrentItem(i)
			if sl.onSelectionChange != nil {
				sl.onSelectionChange(*item.server)
			}
			break
		}
	}
}

func (sl *ServerList) addItemToList(item displayItem) {
	if item.isGroup {
		expanded := sl.groupExpanded[item.group]
		indicator := "▸"
		if expanded {
			indicator = "▾"
		}
		groupName := formatGroupName(item.group)
		primary := fmt.Sprintf("[::b]%s %s[-]", groupName, indicator)
		sl.List.AddItem(primary, "", 0, nil)
	} else if item.server != nil {
		primary, secondary := formatServerLine(*item.server)
		sl.List.AddItem(primary, secondary, 0, nil)
	}
}

// formatGroupName strips the .conf extension and provides a friendly name for display.
func formatGroupName(name string) string {
	if name == domain.SourceFileMain {
		return "config (main)"
	}
	return strings.TrimSuffix(name, ".conf")
}

func (sl *ServerList) UpdateServers(servers []domain.Server) {
	sl.rebuilding = true
	defer func() { sl.rebuilding = false }()

	sl.servers = servers

	// Group servers by SourceFile
	groups := make(map[string][]domain.Server)
	for _, server := range servers {
		sourceFile := server.SourceFile
		if sourceFile == "" {
			sourceFile = domain.SourceFileMain
		}
		groups[sourceFile] = append(groups[sourceFile], server)
	}

	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	// Only initialize expansion state for NEW groups; preserve existing state
	// NEW GROUPS START COLLAPSED by default
	for _, groupName := range groupNames {
		if _, exists := sl.groupExpanded[groupName]; !exists {
			sl.groupExpanded[groupName] = false
		}
	}

	// Remove expansion state for groups that no longer exist
	for name := range sl.groupExpanded {
		found := false
		for _, gn := range groupNames {
			if gn == name {
				found = true
				break
			}
		}
		if !found {
			delete(sl.groupExpanded, name)
		}
	}

	// Build display items
	sl.displayItems = nil
	sl.List.Clear()
	for _, groupName := range groupNames {
		sl.displayItems = append(sl.displayItems, displayItem{
			isGroup: true,
			group:   groupName,
		})
		if sl.groupExpanded[groupName] {
			for _, server := range groups[groupName] {
				sl.displayItems = append(sl.displayItems, displayItem{
					isGroup: false,
					server:  &server,
				})
			}
		}
	}

	for _, item := range sl.displayItems {
		sl.addItemToList(item)
	}

	// Auto-select first server item (skip group headers)
	for i, item := range sl.displayItems {
		if !item.isGroup && item.server != nil {
			sl.List.SetCurrentItem(i)
			if sl.onSelectionChange != nil {
				sl.onSelectionChange(*item.server)
			}
			break
		}
	}
}

func (sl *ServerList) GetSelectedServer() (domain.Server, bool) {
	index := sl.List.GetCurrentItem()
	if index >= 0 && index < len(sl.displayItems) {
		item := sl.displayItems[index]
		if !item.isGroup && item.server != nil {
			return *item.server, true
		}
	}
	return domain.Server{}, false
}

// findServerAt returns the display index if the item at position i is a server (not a group).
func (sl *ServerList) findServerAt(i int) (int, bool) {
	if i >= 0 && i < len(sl.displayItems) && !sl.displayItems[i].isGroup {
		return i, true
	}
	return i, false
}

func (sl *ServerList) OnSelection(fn func(server domain.Server)) *ServerList {
	sl.onSelection = fn
	return sl
}

func (sl *ServerList) OnSelectionChange(fn func(server domain.Server)) *ServerList {
	sl.onSelectionChange = fn
	return sl
}

func (sl *ServerList) OnReturnToSearch(fn func()) *ServerList {
	sl.onReturnToSearch = fn
	return sl
}

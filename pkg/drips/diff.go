package drips

import (
	"sort"
	"strings"
)

// DiffType specifies the kind of modification between local config and on-chain state.
type DiffType string

const (
	DiffAdded     DiffType = "ADDED"     // Present in local config, not on-chain
	DiffRemoved   DiffType = "REMOVED"   // Present on-chain, not in local config
	DiffModified  DiffType = "MODIFIED"  // Present in both, but percentage changed
	DiffUnchanged DiffType = "UNCHANGED" // Present in both with identical percentage
)

// SplitDiffItem represents a single entry in a state diff comparison.
type SplitDiffItem struct {
	Type          DiffType `json:"type"`
	Identifier    string   `json:"identifier"`              // Dependency name or account/address
	Address       string   `json:"address,omitempty"`       // Ethereum address
	OldPercentage int      `json:"old_percentage"`          // Previous on-chain percentage (0 if ADDED)
	NewPercentage int      `json:"new_percentage"`          // Desired local percentage (0 if REMOVED)
}

// SplitsDiff contains the complete comparison between a local DripsConfigFile
// and the live on-chain state from the Drips Subgraph.
type SplitsDiff struct {
	ProjectID      string          `json:"project_id"`
	ChainID        int             `json:"chain_id"`
	Items          []SplitDiffItem `json:"items"`
	AddedCount     int             `json:"added_count"`
	RemovedCount   int             `json:"removed_count"`
	ModifiedCount  int             `json:"modified_count"`
	UnchangedCount int             `json:"unchanged_count"`
}

// HasChanges returns true if there are any ADDED, REMOVED, or MODIFIED items.
func (d *SplitsDiff) HasChanges() bool {
	if d == nil {
		return false
	}
	return (d.AddedCount + d.RemovedCount + d.ModifiedCount) > 0
}

// CalculateSplitsDiff compares local split definitions against on-chain split rules
// and produces a Git-style diff report.
func CalculateSplitsDiff(projectID string, chainID int, localSplits []SplitConfig, onChainSplits []OnChainSplitEntry) *SplitsDiff {
	diff := &SplitsDiff{
		ProjectID: projectID,
		ChainID:   chainID,
		Items:     make([]SplitDiffItem, 0, len(localSplits)+len(onChainSplits)),
	}

	// Index on-chain splits by AccountID / lowercase Address for fast lookup.
	type onChainRef struct {
		entry   OnChainSplitEntry
		matched bool
	}
	onChainMap := make(map[string]*onChainRef)
	for _, oc := range onChainSplits {
		ref := &onChainRef{entry: oc}
		if oc.ReceiverAccountID != "" {
			onChainMap[oc.ReceiverAccountID] = ref
		}
		if oc.ReceiverAddress != "" {
			onChainMap[strings.ToLower(oc.ReceiverAddress)] = ref
		}
	}

	// Process local splits (Added, Modified, Unchanged).
	for _, local := range localSplits {
		id := local.DependencyName
		if id == "" {
			id = local.DripsAccountID
		}
		if id == "" {
			id = local.Address
		}

		// Try matching against on-chain map.
		var matched *onChainRef
		if local.DripsAccountID != "" {
			matched = onChainMap[local.DripsAccountID]
		}
		if matched == nil && local.Address != "" {
			matched = onChainMap[strings.ToLower(local.Address)]
		}
		if matched == nil {
			matched = onChainMap[local.DependencyName]
		}

		if matched == nil {
			// Added in local config
			diff.Items = append(diff.Items, SplitDiffItem{
				Type:          DiffAdded,
				Identifier:    id,
				Address:       local.Address,
				OldPercentage: 0,
				NewPercentage: local.Percentage,
			})
			diff.AddedCount++
		} else {
			matched.matched = true
			oldPct := matched.entry.Percentage
			newPct := local.Percentage

			item := SplitDiffItem{
				Identifier:    id,
				Address:       local.Address,
				OldPercentage: oldPct,
				NewPercentage: newPct,
			}
			if oldPct == newPct {
				item.Type = DiffUnchanged
				diff.UnchangedCount++
			} else {
				item.Type = DiffModified
				diff.ModifiedCount++
			}
			diff.Items = append(diff.Items, item)
		}
	}

	// Process remaining unmatched on-chain splits (Removed).
	for _, oc := range onChainSplits {
		var isMatched bool
		if oc.ReceiverAccountID != "" {
			if ref, ok := onChainMap[oc.ReceiverAccountID]; ok && ref.matched {
				isMatched = true
			}
		}
		if !isMatched && oc.ReceiverAddress != "" {
			if ref, ok := onChainMap[strings.ToLower(oc.ReceiverAddress)]; ok && ref.matched {
				isMatched = true
			}
		}
		if isMatched {
			continue
		}

		id := oc.ReceiverAccountID
		if id == "" {
			id = oc.ReceiverAddress
		}
		diff.Items = append(diff.Items, SplitDiffItem{
			Type:          DiffRemoved,
			Identifier:    id,
			Address:       oc.ReceiverAddress,
			OldPercentage: oc.Percentage,
			NewPercentage: 0,
		})
		diff.RemovedCount++
	}

	// Sort diff items for deterministic display: Added -> Modified -> Removed -> Unchanged.
	sort.Slice(diff.Items, func(i, j int) bool {
		order := map[DiffType]int{
			DiffAdded:     0,
			DiffModified:  1,
			DiffRemoved:   2,
			DiffUnchanged: 3,
		}
		if order[diff.Items[i].Type] != order[diff.Items[j].Type] {
			return order[diff.Items[i].Type] < order[diff.Items[j].Type]
		}
		return diff.Items[i].Identifier < diff.Items[j].Identifier
	})

	return diff
}

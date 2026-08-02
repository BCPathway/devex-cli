package stellar

import (
	"sort"
	"strings"
)

// DiffType specifies the kind of modification between local config and known state.
type DiffType string

const (
	DiffAdded     DiffType = "ADDED"     // Present in local config, not previously known
	DiffRemoved   DiffType = "REMOVED"   // Previously known, not in local config
	DiffModified  DiffType = "MODIFIED"  // Present in both, but percentage changed
	DiffUnchanged DiffType = "UNCHANGED" // Present in both with identical percentage
)

// StellarSplitDiffItem represents a single entry in a state diff comparison.
type StellarSplitDiffItem struct {
	Type          DiffType `json:"type"`
	Identifier    string   `json:"identifier"`              // Dependency name or Stellar address
	Address       string   `json:"address,omitempty"`       // Stellar address (G...)
	OldPercentage int      `json:"old_percentage"`          // Previous percentage (0 if ADDED)
	NewPercentage int      `json:"new_percentage"`          // Desired local percentage (0 if REMOVED)
}

// StellarSplitsDiff contains the complete comparison between a local StellarConfigFile
// and a previous/known state.
type StellarSplitsDiff struct {
	AccountID      string                 `json:"account_id"`
	Network        string                 `json:"network"`
	Items          []StellarSplitDiffItem  `json:"items"`
	AddedCount     int                    `json:"added_count"`
	RemovedCount   int                    `json:"removed_count"`
	ModifiedCount  int                    `json:"modified_count"`
	UnchangedCount int                    `json:"unchanged_count"`
}

// HasChanges returns true if there are any ADDED, REMOVED, or MODIFIED items.
func (d *StellarSplitsDiff) HasChanges() bool {
	if d == nil {
		return false
	}
	return (d.AddedCount + d.RemovedCount + d.ModifiedCount) > 0
}

// CalculateStellarSplitsDiff compares local split definitions against a previous
// set of splits and produces a Git-style diff report.
func CalculateStellarSplitsDiff(accountID, networkName string, localSplits []StellarSplitConfig, previousSplits []StellarSplitConfig) *StellarSplitsDiff {
	diff := &StellarSplitsDiff{
		AccountID: accountID,
		Network:   networkName,
		Items:     make([]StellarSplitDiffItem, 0, len(localSplits)+len(previousSplits)),
	}

	// Index previous splits by dependency name and address for fast lookup.
	type prevRef struct {
		entry   StellarSplitConfig
		matched bool
	}
	prevMap := make(map[string]*prevRef)
	for i, prev := range previousSplits {
		ref := &prevRef{entry: prev}
		if prev.DependencyName != "" {
			prevMap[prev.DependencyName] = ref
		}
		if prev.StellarAddress != "" {
			prevMap[strings.ToUpper(prev.StellarAddress)] = ref
		}
		// Also index by position as fallback.
		prevMap[string(rune('0'+i))] = ref
	}

	// Process local splits (Added, Modified, Unchanged).
	for _, local := range localSplits {
		id := local.DependencyName
		if id == "" {
			id = local.StellarAddress
		}

		// Try matching against previous map.
		var matched *prevRef
		if local.DependencyName != "" {
			matched = prevMap[local.DependencyName]
		}
		if matched == nil && local.StellarAddress != "" {
			matched = prevMap[strings.ToUpper(local.StellarAddress)]
		}

		if matched == nil {
			// Added in local config.
			diff.Items = append(diff.Items, StellarSplitDiffItem{
				Type:          DiffAdded,
				Identifier:    id,
				Address:       local.StellarAddress,
				OldPercentage: 0,
				NewPercentage: local.Percentage,
			})
			diff.AddedCount++
		} else {
			matched.matched = true
			oldPct := matched.entry.Percentage
			newPct := local.Percentage

			item := StellarSplitDiffItem{
				Identifier:    id,
				Address:       local.StellarAddress,
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

	// Process remaining unmatched previous splits (Removed).
	for _, prev := range previousSplits {
		var isMatched bool
		if prev.DependencyName != "" {
			if ref, ok := prevMap[prev.DependencyName]; ok && ref.matched {
				isMatched = true
			}
		}
		if !isMatched && prev.StellarAddress != "" {
			if ref, ok := prevMap[strings.ToUpper(prev.StellarAddress)]; ok && ref.matched {
				isMatched = true
			}
		}
		if isMatched {
			continue
		}

		id := prev.DependencyName
		if id == "" {
			id = prev.StellarAddress
		}
		diff.Items = append(diff.Items, StellarSplitDiffItem{
			Type:          DiffRemoved,
			Identifier:    id,
			Address:       prev.StellarAddress,
			OldPercentage: prev.Percentage,
			NewPercentage: 0,
		})
		diff.RemovedCount++
	}

	// Sort diff items: Added -> Modified -> Removed -> Unchanged.
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

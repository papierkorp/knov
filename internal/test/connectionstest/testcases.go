package connectionstest

import (
	"fmt"
	"slices"

	"knov/internal/files"
	"knov/internal/search"
	"knov/internal/test"
)

// caseParentsAncestors covers handleAPIGetParents/handleAPIGetAncestors, both plain field
// reads off *files.Metadata - childFile's Parents/Ancestor were computed by the real
// parent-child cascade during seeding (parentChildFanOut/updateAncestors),
// not faked.
func caseParentsAncestors() test.CaseResult {
	name := "parents-ancestors"

	child, err := files.MetaDataGet(withPrefix(childFile))
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Equal(child.Parents, []string{withPrefix(parentFile)}) &&
		slices.Equal(child.Ancestor, []string{withPrefix(parentFile)})

	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("parents=ancestor=[%s]", withPrefix(parentFile)),
		Actual:   fmt.Sprintf("parents=%v ancestor=%v", child.Parents, child.Ancestor),
		Success:  success,
	}
	if !success {
		cr.Error = "child's Parents/Ancestor were not computed as expected"
	}
	return cr
}

// caseKidsGrandchildren covers handleAPIGetKids (plain field read) and
// handleAPIGetGrandchildren (internal/server/api_links.go), which has no exported
// equivalent - it loops the parent's Kids and collects each kid's own Kids inline, so
// that loop is replicated here directly.
func caseKidsGrandchildren() test.CaseResult {
	name := "kids-grandchildren"

	parent, err := files.MetaDataGet(withPrefix(parentFile))
	if err != nil {
		return errCase(name, err)
	}

	var grandchildren []string
	for _, kid := range parent.Kids {
		kidMeta, err := files.MetaDataGet(kid)
		if err != nil || kidMeta == nil {
			continue
		}
		grandchildren = append(grandchildren, kidMeta.Kids...)
	}

	success := slices.Contains(parent.Kids, withPrefix(childFile)) && slices.Contains(grandchildren, withPrefix(grandchild))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("parent.Kids contains %s, grandchildren contains %s", withPrefix(childFile), withPrefix(grandchild)),
		Actual:   fmt.Sprintf("kids=%v grandchildren=%v", parent.Kids, grandchildren),
		Success:  success,
	}
	if !success {
		cr.Error = "Kids/grandchildren were not computed/aggregated as expected"
	}
	return cr
}

// caseUsedLinks covers handleAPIGetUsedLinks - linkerFile's UsedLinks was computed by a real
// files.UpdateLinksForSingleFile pass over its markdown content during seeding.
func caseUsedLinks() test.CaseResult {
	name := "used-links"

	linker, err := files.MetaDataGet(withPrefix(linkerFile))
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Contains(linker.UsedLinks, testPath(linkedFile))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("UsedLinks contains %s", testPath(linkedFile)),
		Actual:   fmt.Sprintf("%v", linker.UsedLinks),
		Success:  success,
	}
	if !success {
		cr.Error = "UpdateLinksForSingleFile did not extract the real markdown link as expected"
	}
	return cr
}

// caseLinksToHere covers handleAPIGetLinksToHere - a plain field read off *files.Metadata.
// linkedFile's LinksToHere was populated for real by the linker's files.UpdateLinksForSingleFile
// call during seeding (it updates both the link source's UsedLinks and every linked target's
// LinksToHere), not faked.
func caseLinksToHere() test.CaseResult {
	name := "links-to-here"

	got, err := files.MetaDataGet(withPrefix(linkedFile))
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Equal(got.LinksToHere, []string{withPrefix(linkerFile)})
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("LinksToHere=[%s]", withPrefix(linkerFile)),
		Actual:   fmt.Sprintf("%v", got.LinksToHere),
		Success:  success,
	}
	if !success {
		cr.Error = "LinksToHere did not round-trip through SeedMetadataRaw as expected"
	}
	return cr
}

// caseRelatedFiles covers handleAPIGetRelatedFiles / search.GetRelatedFiles, which just
// truncates the pre-computed Related field to the requested limit.
func caseRelatedFiles() test.CaseResult {
	name := "related-files"

	limited, err := search.GetRelatedFiles(withPrefix(relatedFile), 2)
	if err != nil {
		return errCase(name, err)
	}

	success := len(limited) == 2 && limited[0] == withPrefix(childFile) && limited[1] == withPrefix(grandchild)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("limit=2 truncates Related to [%s, %s]", withPrefix(childFile), withPrefix(grandchild)),
		Actual:   fmt.Sprintf("%v", limited),
		Success:  success,
	}
	if !success {
		cr.Error = "GetRelatedFiles did not truncate the Related list as expected"
	}
	return cr
}

// caseAncestorsInFolder covers handleAPIGetAncestorsInFolder / files.GetAncestorsInFolder -
// the two independent parent-child chains give the test folder two distinct top ancestors.
func caseAncestorsInFolder() test.CaseResult {
	name := "ancestors-in-folder"

	ancestors, err := files.GetAncestorsInFolder(testDir)
	if err != nil {
		return errCase(name, err)
	}

	success := slices.Contains(ancestors, withPrefix(parentFile)) && slices.Contains(ancestors, withPrefix(parent2File))
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("ancestors include both %s and %s", withPrefix(parentFile), withPrefix(parent2File)),
		Actual:   fmt.Sprintf("%v", ancestors),
		Success:  success,
	}
	if !success {
		cr.Error = "GetAncestorsInFolder did not return both distinct chain ancestors"
	}
	return cr
}

// caseConflictBanner covers handleAPIGetConflictBanner's condition (metadata.ConflictFile
// != "") via files.SetConflictFile/ClearConflictFile.
func caseConflictBanner() test.CaseResult {
	name := "conflict-banner"

	if err := files.SetConflictFile(withPrefix(conflictOriginal), withPrefix(conflictCopy)); err != nil {
		return errCase(name, err)
	}
	afterSet, err := files.MetaDataGet(withPrefix(conflictOriginal))
	if err != nil {
		return errCase(name, err)
	}
	setOK := afterSet.ConflictFile == withPrefix(conflictCopy)

	if err := files.ClearConflictFile(withPrefix(conflictOriginal)); err != nil {
		return errCase(name, err)
	}
	afterClear, err := files.MetaDataGet(withPrefix(conflictOriginal))
	if err != nil {
		return errCase(name, err)
	}
	clearOK := afterClear.ConflictFile == ""

	success := setOK && clearOK
	cr := test.CaseResult{
		Name:     name,
		Expected: "ConflictFile set (banner shown) then cleared (banner hidden)",
		Actual:   fmt.Sprintf("afterSet=%q afterClear=%q", afterSet.ConflictFile, afterClear.ConflictFile),
		Success:  success,
	}
	if !success {
		cr.Error = "SetConflictFile/ClearConflictFile did not toggle ConflictFile as expected"
	}
	return cr
}

// caseConflictOfBanner covers handleAPIGetConflictOfBanner's condition (metadata.ConflictOf
// != "") via files.SetConflictOf.
func caseConflictOfBanner() test.CaseResult {
	name := "conflict-of-banner"

	if err := files.SetConflictOf(withPrefix(conflictCopy), withPrefix(conflictOriginal)); err != nil {
		return errCase(name, err)
	}
	got, err := files.MetaDataGet(withPrefix(conflictCopy))
	if err != nil {
		return errCase(name, err)
	}

	success := got.ConflictOf == withPrefix(conflictOriginal)
	cr := test.CaseResult{
		Name:     name,
		Expected: fmt.Sprintf("ConflictOf=%s", withPrefix(conflictOriginal)),
		Actual:   fmt.Sprintf("%q", got.ConflictOf),
		Success:  success,
	}
	if !success {
		cr.Error = "SetConflictOf did not set ConflictOf as expected"
	}
	return cr
}

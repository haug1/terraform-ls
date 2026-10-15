// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"github.com/hashicorp/hcl-lang/decoder"
	"github.com/hashicorp/hcl-lang/reference"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/terraform-ls/internal/features/modules/ast"
	"github.com/hashicorp/terraform-ls/internal/features/modules/state"
)

func modulePathContext(mod *state.ModuleRecord, stateReader CombinedReader) (*decoder.PathContext, error) {
	schema, err := schemaForModule(mod, stateReader)
	if err != nil {
		return nil, err
	}
	functions, err := functionsForModule(mod, stateReader)
	if err != nil {
		return nil, err
	}

	pathCtx := &decoder.PathContext{
		Schema:           schema,
		ReferenceOrigins: make(reference.Origins, 0),
		ReferenceTargets: make(reference.Targets, 0),
		Files:            make(map[string]*hcl.File, 0),
		Functions:        functions,
		Validators:       moduleValidators,
	}

	pathCtx.ReferenceOrigins = moduleReferenceOrigins(mod)
	for _, target := range mod.RefTargets {
		if target.RangePtr != nil && ast.IsModuleFilename(target.RangePtr.Filename) {
			pathCtx.ReferenceTargets = append(pathCtx.ReferenceTargets, target)
		} else if target.RangePtr == nil {
			pathCtx.ReferenceTargets = append(pathCtx.ReferenceTargets, target)
		}
	}

	// append Terraform version specific path targets that are available in all modules (builtin references)
	pathCtx.ReferenceTargets = append(pathCtx.ReferenceTargets, referencesForModule(mod, stateReader)...)

	for name, f := range mod.ParsedModuleFiles {
		pathCtx.Files[name.String()] = f
	}

	return pathCtx, nil
}

// moduleReferenceOrigins returns the reference origins declared in the
// module's own files. Unlike modulePathContext it does not need a schema,
// which makes it cheap enough to call for every module in the workspace.
func moduleReferenceOrigins(mod *state.ModuleRecord) reference.Origins {
	origins := make(reference.Origins, 0)
	for _, origin := range mod.RefOrigins {
		if ast.IsModuleFilename(origin.OriginRange().Filename) {
			origins = append(origins, origin)
		}
	}
	return origins
}

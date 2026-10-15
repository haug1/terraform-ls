// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package codelens

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/hcl-lang/decoder"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/reference"
	"github.com/hashicorp/hcl/v2"
	ilsp "github.com/hashicorp/terraform-ls/internal/lsp"
	lsp "github.com/hashicorp/terraform-ls/internal/protocol"
)

func ReferenceCount(showReferencesCmdId string) lang.CodeLensFunc {
	return func(ctx context.Context, path lang.Path, file string) ([]lang.CodeLens, error) {
		lenses := make([]lang.CodeLens, 0)

		localCtx, err := decoder.PathCtx(ctx)
		if err != nil {
			return nil, err
		}

		pathReader, err := decoder.PathReaderFromContext(ctx)
		if err != nil {
			return nil, err
		}

		refTargets := localCtx.ReferenceTargets.OutermostInFile(file)
		if err != nil {
			return nil, err
		}

		// There can be two targets pointing to the same range
		// e.g. when a block is targetable as type-less reference
		// and as an object, which is important in most contexts
		// but not here, where we present it to the user.
		dedupedTargets := make(map[hcl.Range]reference.Targets, 0)
		for _, refTarget := range refTargets {
			rng := *refTarget.RangePtr
			if _, ok := dedupedTargets[rng]; !ok {
				dedupedTargets[rng] = make(reference.Targets, 0)
			}
			dedupedTargets[rng] = append(dedupedTargets[rng], refTarget)
		}

		// Collect origins of every path once per request, rather than
		// once per target, and without building a full PathContext
		// (incl. provider schemas) where the reader supports that.
		type pathOrigins struct {
			path    lang.Path
			origins reference.Origins
		}
		allOrigins := make([]pathOrigins, 0)
		for _, p := range pathReader.Paths(ctx) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			origins, err := referenceOrigins(pathReader, p)
			if err != nil {
				continue
			}
			allOrigins = append(allOrigins, pathOrigins{path: p, origins: origins})
		}

		for rng, refTargets := range dedupedTargets {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			originCount := 0
			var defRange *hcl.Range
			for _, refTarget := range refTargets {
				if refTarget.DefRangePtr != nil {
					defRange = refTarget.DefRangePtr
				}

				for _, po := range allOrigins {
					originCount += len(po.origins.Match(po.path, refTarget, path))
				}
			}

			if originCount == 0 {
				continue
			}

			var hclPos hcl.Pos
			if defRange != nil {
				hclPos = posMiddleOfRange(defRange)
			} else {
				hclPos = posMiddleOfRange(&rng)
			}

			lenses = append(lenses, lang.CodeLens{
				Range: rng,
				Command: lang.Command{
					Title: getTitle("reference", "references", originCount),
					ID:    showReferencesCmdId,
					Arguments: []lang.CommandArgument{
						Position(ilsp.HCLPosToLSP(hclPos)),
						ReferenceContext(lsp.ReferenceContext{}),
					},
				},
			})
		}

		sort.SliceStable(lenses, func(i, j int) bool {
			return lenses[i].Range.Start.Byte < lenses[j].Range.Start.Byte
		})

		return lenses, nil
	}
}

type referenceOriginsReader interface {
	ReferenceOrigins(path lang.Path) (reference.Origins, error)
}

func referenceOrigins(pathReader decoder.PathReader, p lang.Path) (reference.Origins, error) {
	if or, ok := pathReader.(referenceOriginsReader); ok {
		return or.ReferenceOrigins(p)
	}
	pathCtx, err := pathReader.PathContext(p)
	if err != nil {
		return nil, err
	}
	return pathCtx.ReferenceOrigins, nil
}

type Position lsp.Position

func (p Position) MarshalJSON() ([]byte, error) {
	return json.Marshal(lsp.Position(p))
}

type ReferenceContext lsp.ReferenceContext

func (rc ReferenceContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(lsp.ReferenceContext(rc))
}

func posMiddleOfRange(rng *hcl.Range) hcl.Pos {
	col := rng.Start.Column
	byte := rng.Start.Byte

	if rng.Start.Line == rng.End.Line && rng.End.Column > rng.Start.Column {
		charsFromStart := (rng.End.Column - rng.Start.Column) / 2
		col += charsFromStart
		byte += charsFromStart
	}

	return hcl.Pos{
		Line:   rng.Start.Line,
		Column: col,
		Byte:   byte,
	}
}

func getTitle(singular, plural string, n int) string {
	if n > 1 || n == 0 {
		return fmt.Sprintf("%d %s", n, plural)
	}
	return fmt.Sprintf("%d %s", n, singular)
}

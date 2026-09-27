package decide

import (
	"math/bits"

	"github.com/cespare/xxhash/v2"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	"github.com/faraquic/lotty-ab-platform/pkg/targeting"
	"go.uber.org/zap"
)

const basisPoints = 10000

func bucketPos(experimentID, salt, subjectID string) int {
	sum := xxhash.Sum64String(experimentID + ":" + salt + ":" + subjectID)
	hi, _ := bits.Mul64(sum, basisPoints)
	return int(hi)
}

func selectVariant(exp snapshot.ExperimentSnapshot, pos int) (snapshot.VariantSnapshot, bool) {
	if pos < 0 || pos >= exp.AllocationBp {
		return snapshot.VariantSnapshot{}, false
	}
	acc := 0
	for _, v := range exp.Variants {
		acc += v.WeightBp
		if pos < acc {
			return v, true
		}
	}
	return snapshot.VariantSnapshot{}, false
}

func targetingMatch(targetingExpr []byte, attrs map[string]any, log *zap.Logger) bool {
	match, err := targeting.Evaluate(targetingExpr, attrs)
	if err != nil {
		log.Warn("targeting evaluation failed", zap.Error(err))
		return false
	}
	return match
}

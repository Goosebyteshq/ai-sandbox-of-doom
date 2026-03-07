package engine

import "sort"

type EvalRun struct {
	ID          string  `json:"id"`
	Passed      bool    `json:"passed"`
	RubricScore float64 `json:"rubric_score"`
}

type FlipItem struct {
	ID              string  `json:"id"`
	BaselinePassed  bool    `json:"baseline_passed"`
	CandidatePassed bool    `json:"candidate_passed"`
	BaselineScore   float64 `json:"baseline_score"`
	CandidateScore  float64 `json:"candidate_score"`
	Kind            string  `json:"kind"`
}

type FlipReport struct {
	TotalCompared int        `json:"total_compared"`
	Improved      int        `json:"improved"`
	Regressed     int        `json:"regressed"`
	Unchanged     int        `json:"unchanged"`
	DeltaScoreAvg float64    `json:"delta_score_avg"`
	Items         []FlipItem `json:"items"`
}

func AnalyzeFlips(baseline, candidate []EvalRun) FlipReport {
	baseIndex := map[string]EvalRun{}
	for _, run := range baseline {
		baseIndex[run.ID] = run
	}

	items := []FlipItem{}
	scoreDeltaSum := 0.0
	improved := 0
	regressed := 0
	unchanged := 0
	compared := 0

	for _, cand := range candidate {
		base, ok := baseIndex[cand.ID]
		if !ok {
			continue
		}
		compared++
		item := FlipItem{
			ID:              cand.ID,
			BaselinePassed:  base.Passed,
			CandidatePassed: cand.Passed,
			BaselineScore:   base.RubricScore,
			CandidateScore:  cand.RubricScore,
		}
		scoreDelta := cand.RubricScore - base.RubricScore
		scoreDeltaSum += scoreDelta

		switch {
		case !base.Passed && cand.Passed:
			item.Kind = "fail_to_pass"
			improved++
		case base.Passed && !cand.Passed:
			item.Kind = "pass_to_fail"
			regressed++
		case scoreDelta > 0:
			item.Kind = "score_up"
			improved++
		case scoreDelta < 0:
			item.Kind = "score_down"
			regressed++
		default:
			item.Kind = "unchanged"
			unchanged++
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	deltaAvg := 0.0
	if compared > 0 {
		deltaAvg = scoreDeltaSum / float64(compared)
	}

	return FlipReport{
		TotalCompared: compared,
		Improved:      improved,
		Regressed:     regressed,
		Unchanged:     unchanged,
		DeltaScoreAvg: round2(deltaAvg),
		Items:         items,
	}
}

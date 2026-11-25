package pagelayout

import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"math/bits"
	"slices"

	"github.com/npillmayer/pagelayout/content"
)

const UnitsPerPoint uint64 = 1_000_000

var (
	ErrInvalidCost   = errors.New("invalid cost")
	ErrLossOverflow  = errors.New("loss arithmetic overflow")
	ErrInvalidPolicy = errors.New("invalid loss policy")
)

// Loss is an exact finite cost or a distinct absorbing blocked value. Zero is
// finite zero. Even MaxUint64 finite units compare below blocked.
type Loss struct {
	units   uint64
	blocked bool
}
type LossInfo struct {
	Units   uint64
	Blocked bool
}

func FiniteLoss(units uint64) Loss          { return Loss{units: units} }
func BlockedLoss() Loss                     { return Loss{blocked: true} }
func (l Loss) Info() LossInfo               { return LossInfo{l.units, l.blocked} }
func (l Loss) MarshalJSON() ([]byte, error) { return json.Marshal(l.Info()) }
func (l Loss) Compare(r Loss) int {
	if l.blocked != r.blocked {
		if l.blocked {
			return 1
		}
		return -1
	}
	if l.blocked || l.units == r.units {
		return 0
	}
	if l.units < r.units {
		return -1
	}
	return 1
}
func (l Loss) Add(r Loss) (Loss, error) {
	if l.blocked || r.blocked {
		return BlockedLoss(), nil
	}
	sum, carry := bits.Add64(l.units, r.units, 0)
	if carry != 0 {
		return Loss{}, ErrLossOverflow
	}
	return FiniteLoss(sum), nil
}

// QuantizeLoss rounds a complete weighted component in units, ties to even.
// Integer coefficients/counts should use exact integer arithmetic instead.
func QuantizeLoss(units float64) (Loss, error) {
	if math.IsNaN(units) || math.IsInf(units, 0) || units < 0 {
		return Loss{}, ErrInvalidCost
	}
	return quantizeRat(new(big.Rat).SetFloat64(units))
}
func quantizeRat(r *big.Rat) (Loss, error) {
	if r == nil || r.Sign() < 0 {
		return Loss{}, ErrInvalidCost
	}
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	twice := new(big.Int).Lsh(rem, 1)
	c := twice.Cmp(r.Denom())
	if c > 0 || c == 0 && q.Bit(0) == 1 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsUint64() {
		return Loss{}, ErrLossOverflow
	}
	return FiniteLoss(q.Uint64()), nil
}
func weightedCount(weight, count uint64) (Loss, error) {
	hi, lo := bits.Mul64(weight, count)
	if hi != 0 {
		return Loss{}, ErrLossOverflow
	}
	return FiniteLoss(lo), nil
}

// LossPolicy contains unit coefficients. Defaults are calibration hypotheses;
// representation, checked arithmetic and the positive page lower bound are fixed.
type LossPolicy struct{ Page, Underfill, OneLineSplit, Carry, MissingMain uint64 }

func DefaultLossPolicy() LossPolicy {
	return LossPolicy{UnitsPerPoint, UnitsPerPoint, 10 * UnitsPerPoint, UnitsPerPoint, UnitsPerPoint}
}
func (p LossPolicy) Validate() error {
	if p.Page == 0 {
		return ErrInvalidPolicy
	}
	return nil
}
func (p LossPolicy) MinimumPageLoss() (Loss, error) {
	if err := p.Validate(); err != nil {
		return Loss{}, err
	}
	return FiniteLoss(p.Page), nil
}

// MinimumFinalLoss is a strict-pruning lower bound for an unfinished sequence.
func (p LossPolicy) MinimumFinalLoss(accumulated Loss) (Loss, error) {
	minimum, err := p.MinimumPageLoss()
	if err != nil {
		return Loss{}, err
	}
	return accumulated.Add(minimum)
}

// FrameAssessment describes a receiving rectangle. Occupied is the furthest
// allocated block extent, including surviving edges/spacing, not glyph ink.
type FrameAssessment struct {
	Channel                 string
	Width, Height, Occupied float64
}
type PendingTarget struct {
	ID      BranchID
	Channel string
}
type ParagraphFragment struct {
	Paragraph content.Box
	Frame     int
	Lines     uint64
	Split     bool
}

// PageAssessment contains facts, not weights or a claim that a page is feasible.
// Frames follow chain order within each channel. PendingAfter includes newly
// activated incomplete targets; PendingAtEntry fixes underfill participation.
// Adjacent paragraph pieces in one frame are coalesced before counting penalties.
type PageAssessment struct {
	Frames                       []FrameAssessment
	MainRemaining                bool
	PendingAtEntry, PendingAfter []PendingTarget
	Fragments                    []ParagraphFragment
	MainFinishes                 bool
	MainFinishingFrame           int
}
type Score struct {
	Total, Page, Underfill, Split, Carry, MissingMain Loss
	Participating, Exempt                             []int
	AreaDenominator                                   float64
	Facts                                             PageAssessment
}

func cloneAssessment(a PageAssessment) PageAssessment {
	a.Frames = slices.Clone(a.Frames)
	a.PendingAtEntry = slices.Clone(a.PendingAtEntry)
	a.PendingAfter = slices.Clone(a.PendingAfter)
	a.Fragments = slices.Clone(a.Fragments)
	return a
}

// AssessPage evaluates the provisional scoring policy, never modifies facts,
// clamps geometry or silently ignores overflow. Failure returns no partial score.
func AssessPage(p LossPolicy, a PageAssessment) (Score, error) {
	if err := p.Validate(); err != nil {
		return Score{}, err
	}
	bad := func() (Score, error) { return Score{}, ErrInvalidCost }
	pendingChannels := map[string]bool{}
	targetChannels := map[BranchID]string{}
	for _, list := range [][]PendingTarget{a.PendingAtEntry, a.PendingAfter} {
		for _, target := range list {
			if target.ID == 0 || target.Channel == "" || target.Channel == "main" {
				return bad()
			}
			if old, ok := targetChannels[target.ID]; ok && old != target.Channel {
				return bad()
			}
			targetChannels[target.ID] = target.Channel
		}
	}
	for _, target := range a.PendingAtEntry {
		pendingChannels[target.Channel] = true
	}
	if a.MainFinishes && (!a.MainRemaining || a.MainFinishingFrame < 0 || a.MainFinishingFrame >= len(a.Frames) || a.Frames[a.MainFinishingFrame].Channel != "main") {
		return bad()
	}
	result := Score{Page: FiniteLoss(p.Page), Facts: cloneAssessment(a)}
	denominator, numerator := new(big.Rat), new(big.Rat)
	mainReceiver := false
	for i, f := range a.Frames {
		if f.Channel == "" || !finiteNumber(f.Width) || !finiteNumber(f.Height) || !finiteNumber(f.Occupied) || f.Width <= 0 || f.Height <= 0 || f.Occupied < 0 || f.Occupied > f.Height {
			return bad()
		}
		if f.Channel == "main" {
			mainReceiver = true
		}
		participates := f.Channel == "main" && a.MainRemaining || pendingChannels[f.Channel]
		if !participates {
			continue
		}
		result.Participating = append(result.Participating, i)
		area := new(big.Rat).Mul(new(big.Rat).SetFloat64(f.Width), new(big.Rat).SetFloat64(f.Height))
		denominator.Add(denominator, area)
		if a.MainFinishes && f.Channel == "main" && i >= a.MainFinishingFrame {
			result.Exempt = append(result.Exempt, i)
			continue
		}
		fraction := new(big.Rat).Sub(new(big.Rat).SetFloat64(f.Height), new(big.Rat).SetFloat64(f.Occupied))
		fraction.Quo(fraction, new(big.Rat).SetFloat64(f.Height))
		fraction.Mul(fraction, fraction)
		fraction.Mul(fraction, area)
		numerator.Add(numerator, fraction)
	}
	var err error
	if denominator.Sign() > 0 {
		result.AreaDenominator, _ = denominator.Float64()
		if !finiteNumber(result.AreaDenominator) {
			return bad()
		}
		numerator.Quo(numerator, denominator)
		numerator.Mul(numerator, new(big.Rat).SetInt(new(big.Int).SetUint64(p.Underfill)))
		result.Underfill, err = quantizeRat(numerator)
		if err != nil {
			return Score{}, err
		}
	}
	var count uint64
	for i := 0; i < len(a.Fragments); {
		f := a.Fragments[i]
		if !f.Paragraph.Valid() || f.Frame < 0 || f.Frame >= len(a.Frames) {
			return bad()
		}
		lines, split := f.Lines, f.Split
		j := i + 1
		for j < len(a.Fragments) && a.Fragments[j].Paragraph == f.Paragraph && a.Fragments[j].Frame == f.Frame {
			var carry uint64
			lines, carry = bits.Add64(lines, a.Fragments[j].Lines, 0)
			if carry != 0 {
				return Score{}, ErrLossOverflow
			}
			split = split || a.Fragments[j].Split
			j++
		}
		if split && lines == 1 {
			count++
		}
		i = j
	}
	result.Split, err = weightedCount(p.OneLineSplit, count)
	if err != nil {
		return Score{}, err
	}
	pending := map[BranchID]bool{}
	for _, target := range a.PendingAfter {
		pending[target.ID] = true
	}
	result.Carry, err = weightedCount(p.Carry, uint64(len(pending)))
	if err != nil {
		return Score{}, err
	}
	if a.MainRemaining && !mainReceiver {
		result.MissingMain = FiniteLoss(p.MissingMain)
	}
	for _, part := range []Loss{result.Page, result.Underfill, result.Split, result.Carry, result.MissingMain} {
		result.Total, err = result.Total.Add(part)
		if err != nil {
			return Score{}, err
		}
	}
	return result, nil
}
func finiteNumber(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

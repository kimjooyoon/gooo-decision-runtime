package pathplan

import "fmt"

// ConditionFailure retains one committed counterexample, not an attempt log.
// Its mask and exact input refer to the frozen source plan and finite contract.
type ConditionFailure struct {
	Mask   uint16          `json:"choice_mask"`
	Result ConditionResult `json:"result"`
}

func (session *Session) rememberConditionFailure(attempt SearchAttempt) {
	if session.firstConditionFailure != nil {
		return
	}
	for _, result := range attempt.Conditions {
		if !result.Passed {
			session.firstConditionFailure = &ConditionFailure{Mask: attempt.Mask, Result: result}
			return
		}
	}
}

func (session *Session) bindConditionFeedback(receipt *FeedbackReceipt) {
	receipt.ConditionRejected = session.result.ConditionRejected
	if session.firstConditionFailure != nil {
		copy := *session.firstConditionFailure
		receipt.FirstConditionFailure = &copy
	}
}

// Absence adds no bytes to older model inputs. Unreached conditions explicitly
// have no observed Boolean, even when their zero-value storage contains false.
func conditionFeedbackSuffix(receipt FeedbackReceipt) string {
	if receipt.ConditionRejected == 0 {
		return ""
	}
	text := fmt.Sprintf(" condition_rejected=%d", receipt.ConditionRejected)
	if failure := receipt.FirstConditionFailure; failure != nil {
		row := failure.Result
		actual := "UNOBSERVED"
		if row.Observation.Reached {
			actual = fmt.Sprint(row.Observation.Value)
		}
		text += fmt.Sprintf(" condition_mask=%d condition_choice=%q condition_input=%d condition_expected=%t condition_actual=%s condition_status=%s",
			failure.Mask, row.Case.ChoiceID, row.Case.Input, row.Case.Expected, actual, row.Status)
	}
	return text
}

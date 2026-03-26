package tools

type ApprovalRequest struct {
	Command string
	Reason  string
}

type ApprovalHandler func(ApprovalRequest) (bool, error)

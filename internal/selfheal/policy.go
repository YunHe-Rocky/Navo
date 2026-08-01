package selfheal

import "context"

type PolicyFuncs struct {
	PolicyName   string
	Def          Definition
	CheckFunc    func(context.Context, ErrorEvent) (bool, error)
	RepairFunc   func(context.Context, ErrorEvent) (RepairAction, error)
	VerifyFunc   func(context.Context, ErrorEvent, RepairAction) (VerificationResult, error)
	RollbackFunc func(context.Context, ErrorEvent, RepairAction) error
}

func (p PolicyFuncs) Name() string           { return p.PolicyName }
func (p PolicyFuncs) Definition() Definition { return p.Def }

func (p PolicyFuncs) FaultPresent(ctx context.Context, event ErrorEvent) (bool, error) {
	if p.CheckFunc == nil {
		return false, nil
	}
	return p.CheckFunc(ctx, event)
}

func (p PolicyFuncs) Repair(ctx context.Context, event ErrorEvent) (RepairAction, error) {
	if p.RepairFunc == nil {
		return RepairAction{}, nil
	}
	return p.RepairFunc(ctx, event)
}

func (p PolicyFuncs) Verify(ctx context.Context, event ErrorEvent, action RepairAction) (VerificationResult, error) {
	if p.VerifyFunc == nil {
		return VerificationResult{}, nil
	}
	return p.VerifyFunc(ctx, event, action)
}

func (p PolicyFuncs) Rollback(ctx context.Context, event ErrorEvent, action RepairAction) error {
	if p.RollbackFunc == nil {
		return nil
	}
	return p.RollbackFunc(ctx, event, action)
}

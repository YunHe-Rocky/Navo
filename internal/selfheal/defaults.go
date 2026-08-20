package selfheal

// DefaultObserverPolicies registers ownership boundaries and security blocks.
// Core restarts remain owned by Supervisor; security/privacy failures never
// receive an automatic action.
func DefaultObserverPolicies() []Policy {
	return []Policy{
		observerPolicy("supervisor-core-crash", Definition{
			Code: CodeCoreCrashed, Category: CategoryCore, FaultDomain: FaultDomainCore, Severity: SeverityError,
			Retryable: true, AutoRepair: false,
			Budget: Budget{MaxAttempts: MaxRepairRounds},
		}),
		observerPolicy("subscription-parse-preserve-snapshot", Definition{
			Code: CodeSubscriptionParse, Category: CategorySubscription, FaultDomain: FaultDomainNode, Severity: SeverityError,
			Retryable: false, AutoRepair: false,
		}),
		observerPolicy("privacy-reset-startup-block", Definition{
			Code: CodePrivacyResetFailed, Category: CategorySecurity, FaultDomain: FaultDomainUnknown, Severity: SeverityFatal,
			Retryable: false, AutoRepair: false,
		}),
	}
}

func observerPolicy(name string, definition Definition) Policy {
	return PolicyFuncs{PolicyName: name, Def: definition}
}

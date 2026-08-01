package selfheal

// DefaultObserverPolicies registers ownership boundaries and security blocks.
// Core restarts remain owned by Supervisor; security/privacy failures never
// receive an automatic action.
func DefaultObserverPolicies() []Policy {
	return []Policy{
		observerPolicy("supervisor-core-crash", Definition{
			Code: CodeCoreCrashed, Category: CategoryCore, Severity: SeverityError,
			Retryable: true, AutoRepair: false,
			Budget: Budget{MaxAttempts: 3},
		}),
		observerPolicy("subscription-parse-preserve-snapshot", Definition{
			Code: CodeSubscriptionParse, Category: CategorySubscription, Severity: SeverityError,
			Retryable: false, AutoRepair: false,
		}),
		observerPolicy("privacy-reset-startup-block", Definition{
			Code: CodePrivacyResetFailed, Category: CategorySecurity, Severity: SeverityFatal,
			Retryable: false, AutoRepair: false,
		}),
	}
}

func observerPolicy(name string, definition Definition) Policy {
	return PolicyFuncs{PolicyName: name, Def: definition}
}

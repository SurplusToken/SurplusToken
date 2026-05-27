package service

func surplusAITestDefaultOAuthAccount(account Account) Account {
	if account.Type == "" {
		account.Type = AccountTypeOAuth
	}
	return account
}

func surplusAITestDefaultOAuthAccountPtr(account *Account) *Account {
	if account == nil || account.Type != "" {
		return account
	}
	copied := *account
	copied.Type = AccountTypeOAuth
	return &copied
}

func surplusAITestDefaultOAuthAccounts(accounts []Account) []Account {
	if len(accounts) == 0 {
		return accounts
	}
	out := make([]Account, len(accounts))
	for i := range accounts {
		out[i] = surplusAITestDefaultOAuthAccount(accounts[i])
	}
	return out
}

package handler

// clerkWebhookEvent is the payload of a Clerk webhook event.
type clerkWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		ID                    string `json:"id"`
		PrimaryEmailAddressID string `json:"primary_email_address_id"`
		DeletedAt             string `json:"deleted_at"`
		EmailAddresses        []struct {
			ID           string `json:"id"`
			EmailAddress string `json:"email_address"`
		} `json:"email_addresses"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"data"`
}

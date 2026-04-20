package samgov

// SetBaseURLForTest lets external tests (same module, other package) override
// the client's base URL to point at an httptest server.
func SetBaseURLForTest(c *Client, u string) { c.baseURL = u }

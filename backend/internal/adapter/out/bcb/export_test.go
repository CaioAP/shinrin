package bcb

import "time"

// SetBackoff shortens the HTML-page retry wait in tests.
func (c *Client) SetBackoff(d time.Duration) { c.backoff = d }

package moodle

import (
	"context"
)

func (c *Client) Discovery(ctx context.Context, courseID string) (int, error) {
	_, sections, err := c.CourseContents(ctx, courseID)
	if err != nil {
		return 0, err
	}
	assignments, _ := c.Assignments(ctx, courseID)

	total := 0
	for _, s := range sections {
		for _, m := range s.Modules {
			if len(m.Contents) > 0 {
				total += len(m.Contents)
			} else if m.Type == "resource" || m.Type == "file" {
				total++
			}
		}
	}

	total += len(assignments)
	return total, nil
}

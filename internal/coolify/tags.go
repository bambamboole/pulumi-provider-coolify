package coolify

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// Tag is a team-wide tag attached to a resource.
type Tag struct {
	UUID string
	Name string
}

func tagsFromAPI(tags []api.Tag) []Tag {
	out := make([]Tag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, Tag{UUID: Deref(tag.Uuid), Name: Deref(tag.Name)})
	}
	return out
}

// ListTags returns the tags attached to the owner.
func (c *Client) ListTags(ctx context.Context, owner Owner) ([]Tag, error) {
	var resp *http.Response
	var err error
	switch owner.Kind {
	case OwnerApplication:
		resp, err = c.api.ListTagsByApplicationUuid(ctx, owner.UUID)
	case OwnerDatabase:
		resp, err = c.api.ListTagsByDatabaseUuid(ctx, owner.UUID)
	case OwnerService:
		resp, err = c.api.ListTagsByServiceUuid(ctx, owner.UUID)
	default:
		return nil, fmt.Errorf("coolify: unsupported tag owner %q", owner.Kind)
	}
	tags, err := decode[[]api.Tag](resp, err)
	if err != nil {
		return nil, err
	}
	return tagsFromAPI(tags), nil
}

// AttachTags attaches the named tags to the owner, creating tags that do not
// exist in the team yet, and returns the owner's full tag list.
func (c *Client) AttachTags(ctx context.Context, owner Owner, names []string) ([]Tag, error) {
	var resp *http.Response
	var err error
	switch owner.Kind {
	case OwnerApplication:
		resp, err = c.api.CreateTagByApplicationUuid(ctx, owner.UUID, api.CreateTagByApplicationUuidJSONRequestBody{TagNames: &names})
	case OwnerDatabase:
		resp, err = c.api.CreateTagByDatabaseUuid(ctx, owner.UUID, api.CreateTagByDatabaseUuidJSONRequestBody{TagNames: &names})
	case OwnerService:
		resp, err = c.api.CreateTagByServiceUuid(ctx, owner.UUID, api.CreateTagByServiceUuidJSONRequestBody{TagNames: &names})
	default:
		return nil, fmt.Errorf("coolify: unsupported tag owner %q", owner.Kind)
	}
	tags, err := decode[[]api.Tag](resp, err)
	if err != nil {
		return nil, err
	}
	return tagsFromAPI(tags), nil
}

// DetachTag detaches a tag from the owner. Coolify deletes the tag when no
// resource carries it any more.
func (c *Client) DetachTag(ctx context.Context, owner Owner, tagUUID string) error {
	switch owner.Kind {
	case OwnerApplication:
		return check(c.api.DeleteTagByApplicationUuid(ctx, owner.UUID, tagUUID))
	case OwnerDatabase:
		return check(c.api.DeleteTagByDatabaseUuid(ctx, owner.UUID, tagUUID))
	case OwnerService:
		return check(c.api.DeleteTagByServiceUuid(ctx, owner.UUID, tagUUID))
	default:
		return fmt.Errorf("coolify: unsupported tag owner %q", owner.Kind)
	}
}

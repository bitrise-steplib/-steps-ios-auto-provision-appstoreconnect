package spaceship

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bitrise-steplib/steps-ios-auto-provision-appstoreconnect/appstoreconnect"
	"github.com/bitrise-steplib/steps-ios-auto-provision-appstoreconnect/autoprovision"
)

// Profile ...
type Profile struct {
	attributes     appstoreconnect.ProfileAttributes
	id             string
	bundleID       string
	deviceIDs      []string
	certificateIDs []string
}

// ID ...
func (p Profile) ID() string {
	return p.id
}

// Attributes ...
func (p Profile) Attributes() appstoreconnect.ProfileAttributes {
	return p.attributes
}

// CertificateIDs ...
func (p Profile) CertificateIDs() (map[string]bool, error) {
	ids := make(map[string]bool)
	for _, id := range p.certificateIDs {
		ids[id] = true
	}

	return ids, nil
}

// DeviceIDs ...
func (p Profile) DeviceIDs() (map[string]bool, error) {
	ids := make(map[string]bool)
	for _, id := range p.deviceIDs {
		ids[id] = true
	}

	return ids, nil
}

// BundleID ...
func (p Profile) BundleID() (appstoreconnect.BundleID, error) {
	return appstoreconnect.BundleID{
		ID: p.id,
		Attributes: appstoreconnect.BundleIDAttributes{
			Identifier: p.bundleID,
			Name:       p.attributes.Name,
		},
	}, nil
}

// ProfileClient ...
type ProfileClient struct {
	client *Client
}

// NewSpaceshipProfileClient ...
func NewSpaceshipProfileClient(client *Client) *ProfileClient {
	return &ProfileClient{client: client}
}

// ProfileInfo ...
type ProfileInfo struct {
	ID           string                           `json:"id"`
	UUID         string                           `json:"uuid"`
	Name         string                           `json:"name"`
	Status       appstoreconnect.ProfileState     `json:"status"`
	Expiry       time.Time                        `json:"expiry"`
	Platform     appstoreconnect.BundleIDPlatform `json:"platform"`
	Content      string                           `json:"content"`
	AppID        string                           `json:"app_id"`
	BundleID     string                           `json:"bundle_id"`
	Certificates []string                         `json:"certificates"`
	Devices      []string                         `json:"devices"`
}

func newProfile(p ProfileInfo) (Profile, error) {
	contents, err := base64.StdEncoding.DecodeString(p.Content)
	if err != nil {
		return Profile{}, fmt.Errorf("failed to decode profile contents: %v", err)
	}

	return Profile{
		attributes: appstoreconnect.ProfileAttributes{
			Name:           p.Name,
			UUID:           p.UUID,
			ProfileState:   appstoreconnect.ProfileState(p.Status),
			ProfileContent: contents,
			Platform:       p.Platform,
			ExpirationDate: appstoreconnect.Time(p.Expiry),
		},
		id:             p.ID,
		bundleID:       p.BundleID,
		certificateIDs: p.Certificates,
		deviceIDs:      p.Devices,
	}, nil
}

// AppInfo ...
type AppInfo struct {
	ID       string `json:"id"`
	BundleID string `json:"bundleID"`
	Name     string `json:"name"`
}

// FindProfile ...
func (c *ProfileClient) FindProfile(name string, profileType appstoreconnect.ProfileType) (autoprovision.Profile, error) {
	cmd, err := c.client.createRequestCommand("list_profiles", "--name", name, "--profile-type", string(profileType))
	if err != nil {
		return nil, err
	}

	output, err := runSpaceshipCommand(cmd)
	if err != nil {
		return nil, err
	}

	var profileResponse struct {
		Data []ProfileInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &profileResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(profileResponse.Data) == 0 {
		return nil, nil
	}

	profile, err := newProfile(profileResponse.Data[0])
	if err != nil {
		return nil, err
	}

	return profile, nil
}

// DeleteExpiredProfile ...
func (c *ProfileClient) DeleteExpiredProfile(bundleID *appstoreconnect.BundleID, profileName string) error {
	return c.DeleteProfile(bundleID.ID)
}

// CheckProfile ...
func (c *ProfileClient) CheckProfile(prof autoprovision.Profile, entitlements autoprovision.Entitlement, deviceIDs, certificateIDs []string, minProfileDaysValid int) error {
	if autoprovision.IsProfileExpired(prof, minProfileDaysValid) {
		return autoprovision.NonmatchingProfileError{
			Reason: fmt.Sprintf("profile expired, or will expire in less then %d day(s)", minProfileDaysValid),
		}
	}

	profileDeviceIDs, err := prof.DeviceIDs()
	if err != nil {
		return err
	}
	for _, id := range deviceIDs {
		if !profileDeviceIDs[id] {
			return autoprovision.NonmatchingProfileError{
				Reason: fmt.Sprintf("device with ID (%s) not included in the profile", id),
			}
		}
	}

	profileCertificateIDs, err := prof.CertificateIDs()
	if err != nil {
		return err
	}
	for _, id := range certificateIDs {
		if !profileCertificateIDs[id] {
			return autoprovision.NonmatchingProfileError{
				Reason: fmt.Sprintf("certificate with ID (%s) not included in the profile", id),
			}
		}
	}

	bundleID, err := prof.BundleID()
	if err != nil {
		return err
	}

	if err := c.CheckBundleIDEntitlements(bundleID, entitlements); err != nil {
		return autoprovision.NonmatchingProfileError{
			Reason: "entitlements are missing",
		}
	}

	return nil
}

// DeleteProfile ...
func (c *ProfileClient) DeleteProfile(id string) error {
	cmd, err := c.client.createRequestCommand("delete_profile", "--id", id)
	if err != nil {
		return err
	}

	_, err = runSpaceshipCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

// CreateProfile ...
func (c *ProfileClient) CreateProfile(name string, profileType appstoreconnect.ProfileType, bundleID appstoreconnect.BundleID, certificateIDs []string, deviceIDs []string) (autoprovision.Profile, error) {
	cmd, err := c.client.createRequestCommand("create_profile",
		"--bundle_id", bundleID.Attributes.Identifier,
		"--certificate", certificateIDs[0],
		"--profile_name", name,
		"--profile-type", string(profileType),
	)
	if err != nil {
		return nil, err
	}

	output, err := runSpaceshipCommand(cmd)
	if err != nil {
		return nil, err
	}

	var profileResponse struct {
		Data ProfileInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &profileResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v (%s)", err, output)
	}

	profile, err := newProfile(profileResponse.Data)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

// FindBundleID ...
func (c *ProfileClient) FindBundleID(bundleIDIdentifier string) (*appstoreconnect.BundleID, error) {
	cmd, err := c.client.createRequestCommand("get_app", "--bundle_id", bundleIDIdentifier)
	if err != nil {
		return nil, err
	}

	output, err := runSpaceshipCommand(cmd)
	if err != nil {
		return nil, err
	}

	var appResponse struct {
		Data AppInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &appResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &appstoreconnect.BundleID{
		ID: appResponse.Data.ID,
		Attributes: appstoreconnect.BundleIDAttributes{
			Identifier: appResponse.Data.BundleID,
			Name:       appResponse.Data.Name,
		},
	}, nil
}

// CreateBundleID ...
func (c *ProfileClient) CreateBundleID(bundleIDIdentifier string) (*appstoreconnect.BundleID, error) {
	cmd, err := c.client.createRequestCommand("create_bundleid", "--bundle_id", bundleIDIdentifier)
	if err != nil {
		return nil, err
	}

	output, err := runSpaceshipCommand(cmd)
	if err != nil {
		return nil, err
	}

	var appResponse struct {
		Data AppInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &appResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &appstoreconnect.BundleID{
		ID: appResponse.Data.ID,
		Attributes: appstoreconnect.BundleIDAttributes{
			Identifier: appResponse.Data.BundleID,
			Name:       appResponse.Data.Name,
		},
	}, nil
}

// CheckBundleIDEntitlements ...
func (c *ProfileClient) CheckBundleIDEntitlements(bundleID appstoreconnect.BundleID, projectEntitlements autoprovision.Entitlement) error {
	entitlementsBytes, err := json.Marshal(projectEntitlements)
	if err != nil {
		return err
	}
	entitlementsBase64 := base64.StdEncoding.EncodeToString(entitlementsBytes)

	cmd, err := c.client.createRequestCommand("check_bundleid", "--bundle_id", bundleID.Attributes.Identifier, "--entitlements", entitlementsBase64)
	if err != nil {
		return err
	}

	_, err = runSpaceshipCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

// SyncBundleID ...
func (c *ProfileClient) SyncBundleID(bundleID appstoreconnect.BundleID, projectEntitlements autoprovision.Entitlement) error {
	entitlementsBytes, err := json.Marshal(projectEntitlements)
	if err != nil {
		return err
	}
	entitlementsBase64 := base64.StdEncoding.EncodeToString(entitlementsBytes)

	cmd, err := c.client.createRequestCommand("sync_bundleid", "--bundle_id", bundleID.Attributes.Identifier, "--entitlements", entitlementsBase64)
	if err != nil {
		return err
	}

	_, err = runSpaceshipCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

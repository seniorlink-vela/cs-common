package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/seniorlink-vela/cs-common/config"
	velacontext "github.com/seniorlink-vela/cs-common/context"
	"github.com/seniorlink-vela/cs-common/validation"
)

var clientTransport *http.Transport
var apiClient *http.Client

type GenderOption string

const (
	GenderFemale      GenderOption = "Female"
	GenderMale        GenderOption = "Male"
	GenderTransgender GenderOption = "Transgender"
	GenderUnspecified GenderOption = "Unspecified"
)

type ErrorMap map[string]string

func (em ErrorMap) AppendErrorField(name string, message string) {
	em[name] = message
}

func (em ErrorMap) Error() string {
	return fmt.Sprintf("%#v", em)
}

type CaregiverCreate struct {
	ID      string
	Primary bool
}

func Init(maxIdle int, idleTimeout, clientTimeout time.Duration) {
	clientTransport = &http.Transport{
		DisableKeepAlives: true,
		MaxIdleConns:      maxIdle,
		IdleConnTimeout:   idleTimeout,
	}
	apiClient = &http.Client{
		Timeout:   clientTimeout,
		Transport: clientTransport,
	}
}

type HttpErrorField struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

type HttpClientError struct {
	StatusCode int              `json:"status_code"`
	Path       string           `json:"path"`
	Message    string           `json:"message"`
	ErrorType  string           `json:"error_type"`
	Fields     []HttpErrorField `json:"fields,omitempty"`
}

func (h HttpClientError) Error() string {
	return fmt.Sprintf(
		"status code: %d, path: %s, message: %s, error_type: %s",
		h.StatusCode,
		h.Path,
		h.Message,
		h.ErrorType,
	)
}

type Profile struct {
	ID                   string            `json:"id,omitempty"`
	FirstName            *string           `json:"first_name,omitempty" validation:"required,max-length:255"`
	MiddleName           *string           `json:"middle_name,omitempty" validation:"max-length:255"`
	LastName             *string           `json:"last_name,omitempty" validation:"required,max-length:255"`
	Username             *string           `json:"username,omitempty" validation:"required,max-length:255"`
	Email                *string           `json:"email,omitempty" validation:"email,max-length:255,required"`
	SecondEmail          *string           `json:"second_email,omitempty" validation:"email,max-length:255"`
	AddressLine1         *string           `json:"address1,omitempty" validation:"max-length:255"`
	AddressLine2         *string           `json:"address2,omitempty" validation:"max-length:255"`
	City                 *string           `json:"city,omitempty" validation:"max-length:255"`
	State                *string           `json:"state,omitempty" validation:"max-length:255"`
	ZipCode              *string           `json:"zip_code,omitempty" validation:"max-length:255"`
	Country              *string           `json:"country,omitempty" validation:"max-length:255"`
	PrimaryPhoneNumber   *string           `json:"primary_phone_number,omitempty"`
	PrimaryPhoneType     *string           `json:"primary_phone_type,omitempty" validation:"values-insensitive:mobile|home|work|tablet|other"`
	SecondaryPhoneNumber *string           `json:"secondary_phone_number,omitempty"`
	SecondaryPhoneType   *string           `json:"secondary_phone_type,omitempty" validation:"values-insensitive:mobile|home|work|tablet|other"`
	Locale               *string           `json:"locale,omitempty" validation:"max-length:255"`
	TimeZone             *string           `json:"time_zone,omitempty"`
	Gender               *GenderOption     `json:"gender,omitempty" validation:"values:Female|Male|Transgender|Unspecififed"`
	Birthday             *time.Time        `json:"birthday,omitempty"`
	NeedsOnboarding      bool              `json:"needs_onboarding,omitempty"`
	UserTypeID           *int              `json:"user_type_id"`
	OrganizationID       *int              `json:"organization_id,omitempty"`
	ExtendedProperties   map[string]string `json:"extended_properties,omitempty" pg:"extended_properties,hstore"`
	AccessToken          string            `json:"-"`
	Landing              string            `json:"landing" validation:"required"`
	Program              string            `json:"program" validation:"required"`
	Extensions           *[]*ExtensionData `json:"extensions,omitempty"`
}

type ExtensionData struct {
	ID          int64                       `json:"extension_id" validate:"required"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Values      []*ObjectExtensionDataValue `json:"values"`
}

type ObjectExtensionDataValue struct {
	ExtensionID        int64       `json:"extension_id"`
	FieldQualifiedName string      `json:"field_qualified_name"`
	FieldValue         interface{} `json:"value"`
	Repeating          Repeating   `json:"repeating"`
}

type Repeating struct {
	Index  int  `json:"index"`
	Hidden bool `json:"hidden"`
}

type ProfileResponse struct {
	P Profile `json:"user_profile"`
}

func (p *Profile) Validate() error {
	var validationError = ErrorMap{}
	_ = validation.ValidateStruct(*p, validationError)

	conf := config.Current()

	if _, lOk := conf.Landing[p.Landing]; !lOk {
		validationError.AppendErrorField("landing", "Invalid landing passed")
	} else {
		if _, pOk := conf.Landing[p.Landing].ProgramMap[p.Program]; !pOk {
			validationError.AppendErrorField("program", "Invalid program passed")
		}
	}
	if len(validationError) > 0 {
		return validationError
	}
	return nil
}

type OAuthRequest struct {
	Username string
	Password string
	ClientID string
}

type OAuthResponse struct {
	AccessToken string `json:"access_token"`
}

func (o OAuthRequest) toParams() url.Values {

	params := url.Values{}
	params.Add("grant_type", "password")
	params.Add("client_id", o.ClientID)
	params.Add("username", o.Username)
	params.Add("password", o.Password)
	return params
}

func (o OAuthRequest) GetToken(ctx context.Context, baseURI string) (*OAuthResponse, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	requestID := velacontext.GetContextRequestID(ctx)
	params := o.toParams()
	tokenRequestURI := fmt.Sprintf("%s/authentication/token", baseURI)
	b := strings.NewReader(params.Encode())
	req, err := http.NewRequest("POST", tokenRequestURI, b)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("X-Vela-Request-Id", requestID)
	req.Close = true
	if err != nil {
		return nil, err
	}
	resp, reqErr := apiClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if reqErr != nil {
		return nil, reqErr
	}
	if resp.StatusCode != http.StatusOK {
		var errMap map[string]interface{}
		jsonErr := json.NewDecoder(resp.Body).Decode(&errMap)
		if jsonErr != nil {
			return nil, jsonErr
		}
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("OAuth error", zap.Any("response", errMap))
		return nil, errors.New("Can't log in to oauth")
	}
	oresp := &OAuthResponse{}
	jsonErr := json.NewDecoder(resp.Body).Decode(oresp)
	if jsonErr != nil {
		return nil, jsonErr
	}
	return oresp, nil
}

func (p *Profile) CreateProfile(ctx context.Context) error {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	orgID := conf.Landing[p.Landing].ProgramMap[p.Program].OrganizationID
	userTypeID := conf.Landing[p.Landing].ProgramMap[p.Program].UserTypeID

	p.OrganizationID = &orgID
	p.UserTypeID = &userTypeID

	body := map[string]Profile{
		"user_profile": *p,
	}
	url := fmt.Sprintf("%s/api/v1/admin/user-profiles", conf.Common.PublicBaseURI)
	jsonValue, _ := json.Marshal(body)
	request, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return err
	}
	var dat map[string]interface{}
	data, _ := ioutil.ReadAll(response.Body)
	if err = json.Unmarshal(data, &dat); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("Create profile error", zap.Any("response", dat))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return err
		}
		if errResp.Fields != nil && len(errResp.Fields) > 0 {
			errMap := ErrorMap{}
			for _, f := range errResp.Fields {
				fn := strings.Split(f.Name, ":")
				errMap.AppendErrorField(fn[len(fn)-1], f.Message)
			}
			return errMap
		}
		errResp.Path = url
		return errResp
	}
	inner, _ := dat["user_profile"].(map[string]interface{})
	consumerID, cidok := inner["id"].(string)
	if !cidok || len(consumerID) == 0 {
		return errors.New("Failed to aquire consumer ID")
	}
	p.ID = consumerID
	return nil
}

// GetCareteamID -
func (p *Profile) GetCareRoomID(ctx context.Context) (string, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	url := fmt.Sprintf("%s/api/v1/admin/care-teams/consumer/%s", conf.Common.PublicBaseURI, p.ID)
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return "", err
	}
	data, _ := ioutil.ReadAll(response.Body)

	if response.StatusCode != http.StatusOK {
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return "", err
		}
		errResp.Path = url
		return "", errResp
	}
	var dat map[string]interface{}
	if err = json.Unmarshal(data, &dat); err != nil {
		return "", err
	}
	inner, cidok := dat["care_team"].(map[string]interface{})
	ctID := inner["id"].(float64)
	careTeamID := fmt.Sprintf("%.0f", ctID)
	if !cidok || len(careTeamID) == 0 {
		return "", errors.New("Failed to aquire care team ID")
	}
	return careTeamID, nil
}

//AuthorizeVelaCareteam POST /api/v1/admin/care-teams/{care_team_id}/authorize - Authorize the care team
func (p *Profile) AuthorizeCareRoom(ctx context.Context, careTeamID string) error {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	url := fmt.Sprintf("%s/api/v1/admin/care-teams/%s/authorize", conf.Common.PublicBaseURI, careTeamID)

	jsonMap := map[string]interface{}{
		"authorize": map[string]interface{}{
			"authorized":    true,
			"authorized_at": time.Now().UTC(),
			"authorized_by": p.ID,
		},
	}
	jsonValue, _ := json.Marshal(jsonMap)

	request, rerr := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	response, err := apiClient.Do(request)
	if rerr != nil || err != nil || response == nil {
		return err
	}
	var dat map[string]interface{}
	data, _ := ioutil.ReadAll(response.Body)
	if err = json.Unmarshal(data, &dat); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return err
		}
		errResp.Path = url
		return errResp
	}
	return nil
}

func (p *Profile) AddProfessionals(ctx context.Context, careTeamID string, proIDs []string) error {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	url := fmt.Sprintf("%s/api/v1/admin/care-teams/%s/member", conf.Common.PublicBaseURI, careTeamID)
	newMemberTmpl := `{"member":{"user_id": "%s", "owner_type": "CareManager"}}`

	for _, proID := range proIDs {
		jsonStr := fmt.Sprintf(newMemberTmpl, proID)

		request, rerr := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Add("X-Vela-Request-Id", requestID)
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
		response, err := apiClient.Do(request)
		if rerr != nil || err != nil || response == nil {
			return err
		}
		var dat map[string]interface{}
		data, _ := ioutil.ReadAll(response.Body)
		if err = json.Unmarshal(data, &dat); err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			var errResp HttpClientError
			if err = json.Unmarshal(data, &errResp); err != nil {
				return err
			}
			errResp.Path = url
			return errResp
		}
	}
	return nil
}

func (p *Profile) AddCareGiversToCareTeam(ctx context.Context, careTeamID string, cgs []CaregiverCreate) error {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	url := fmt.Sprintf("%s/api/v1/admin/care-teams/%s/member", conf.Common.PublicBaseURI, careTeamID)
	newMemberTmpl := `{"member":{"user_id": "%s", "owner_type": "Caregiver", "rank": %d}}`

	for _, cg := range cgs {
		rank := 1
		if cg.Primary {
			rank = 0
		}
		jsonStr := fmt.Sprintf(newMemberTmpl, cg.ID, rank)

		request, rerr := http.NewRequest("POST", url, bytes.NewBuffer([]byte(jsonStr)))
		if rerr != nil {
			return rerr
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Add("X-Vela-Request-Id", requestID)
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
		response, err := apiClient.Do(request)
		if rerr != nil || err != nil || response == nil {
			return err
		}
		var dat map[string]interface{}
		data, _ := ioutil.ReadAll(response.Body)
		if err = json.Unmarshal(data, &dat); err != nil {
			return err
		}
		if response.StatusCode != http.StatusOK {
			var errResp HttpClientError
			if err = json.Unmarshal(data, &errResp); err != nil {
				return err
			}
			errResp.Path = url
			return errResp
		}
	}
	return nil
}

// Non-nil error indicates failure of the call; true, nil means you found them, false, nil means they were not found
// Updates the Profile with values returned from the call
// Could also pass in the conf - but I stayed with existing pattern
func (p *Profile) UserExistsForEmail(ctx context.Context, token string, email string) (bool, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)
	url := fmt.Sprintf("%s/api/v1/admin/user-profiles/by-reference/email/%s", conf.Common.PublicBaseURI, email)
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return false, err
	}
	data, _ := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if response.StatusCode != http.StatusOK {
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return false, err
		}
		errResp.Path = url
		return false, errResp
	}

	// otherwise we found them so unmarshall into class and return true
	var pr ProfileResponse
	if err = json.Unmarshal(data, &pr); err != nil {
		return false, err
	}

	// assign the returned values into my profile struct
	*p = pr.P
	return true, nil
}

// Returns false/error if not found or error
// When found loads profile into p and returns true
func (p *Profile) GetByID(ctx context.Context, token string, ID string) (bool, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)
	url := fmt.Sprintf("%s/api/v1/admin/user-profiles/%s", conf.Common.PublicBaseURI, ID)
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return false, err
	}
	data, _ := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("Get profile error", zap.Any("response", data))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return false, err
		}
		errResp.Path = url
		return false, errResp
	}

	// otherwise we found them so unmarshall into class and return true
	var pr ProfileResponse
	if err = json.Unmarshal(data, &pr); err != nil {
		return false, err
	}

	// assign the returned values into my profile struct
	*p = pr.P
	return true, nil
}

func (p *Profile) PatchProfile(ctx context.Context, token string) error {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)

	body := map[string]Profile{
		"user_profile": *p,
	}
	if len(p.ID) < 1 {
		return errors.New("No ID to update")
	}
	if len(token) > 0 {
		p.AccessToken = token
	}
	url := fmt.Sprintf("%s/api/v1/admin/user-profiles/%s", conf.Common.PublicBaseURI, p.ID)
	jsonValue, _ := json.Marshal(body)
	request, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.AccessToken))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return err
	}
	var dat map[string]interface{}
	data, _ := ioutil.ReadAll(response.Body)
	if err = json.Unmarshal(data, &dat); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("Patch profile error", zap.Any("response", dat))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return err
		}
		if errResp.Fields != nil && len(errResp.Fields) > 0 {
			errMap := ErrorMap{}
			for _, f := range errResp.Fields {
				fn := strings.Split(f.Name, ":")
				errMap.AppendErrorField(fn[len(fn)-1], f.Message)
			}
			return errMap
		}
		errResp.Path = url
		return errResp
	}
	inner, _ := dat["user_profile"].(map[string]interface{})
	consumerID, cidok := inner["id"].(string)
	if !cidok || len(consumerID) == 0 {
		return errors.New("Failed to aquire consumer ID")
	}
	p.ID = consumerID
	return nil
}

type EventQueue struct {
	ContactEmail     string      `json:"contact_email"`
	CreatedAt        time.Time   `json:"created_at"`
	DisplayName      string      `json:"display_name"`
	UpdatedAt        time.Time   `json:"updated_at"`
	CurrentWatermark int64       `json:"current_watermark"`
	Description      string      `json:"description"`
	EventTypes       []EventType `json:"event_types"`
	ID               int64       `json:"id"`
	MaximumRecords   int64       `json:"maximum_records"`
	Status           string      `json:"status"`
	OrganizationID   int64       `json:"organization_id"`
	PartnerID        int64       `json:"partner_id"`
}

type QueueResponse struct {
	EQ EventQueue `json:"queue"`
}

type EventType struct {
	ID              int64     `json:"id"`
	AvroMessageType string    `json:"avro_message_name"`
	CreatedAt       time.Time `json:"created_at"`
	DisplayName     string    `json:"display_name"`
	UpdatedAt       time.Time `json:"updated_at"`
	Slug            string    `json:"slug"`
}

type EventResponse struct {
	Events        []Event `json:"events"`
	LastReadIndex int64   `json:"last_read_index"`
}

type Event struct {
	CreatedAt        time.Time              `json:"created_at"`
	EventType        string                 `json:"event_type"`
	EventTypeID      int64                  `json:"event_type_id"`
	ID               int64                  `json:"id"`
	MessageSource    string                 `json:"message_source"`
	MessageTimestamp time.Time              `json:"message_timestamp"`
	MessageUUID      string                 `json:"message_uuid"`
	OrganizationID   int64                  `json:"organization_id"`
	PartnerID        int64                  `json:"partner_id"`
	Payload          map[string]interface{} `json:"payload"`
}

type Watermark struct {
	LastReadIndex  int64 `json:"last_read_index"`
	OrganizationID int64 `json:"organization_id,omitempty"`
}

//GET /api/v1/events/queue
func GetQueue(ctx context.Context, token string) (*EventQueue, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)
	url := fmt.Sprintf("%s/api/v1/events/queue", conf.Common.PublicBaseURI)
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return nil, err
	}
	data, _ := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("Get queue error", zap.Any("response", data))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return nil, err
		}
		errResp.Path = url
		return nil, errResp
	}

	// otherwise we found them so unmarshall into class and return true
	var q QueueResponse
	if err = json.Unmarshal(data, &q); err != nil {
		return nil, err
	}

	return &q.EQ, nil
}

// GET /api/v1/events/queue/events
func GetEventsForQueue(ctx context.Context, token string, maxRecords *int64, slugs []string) ([]Event, int64, error) {
	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)
	url := fmt.Sprintf("%s/api/v1/events/queue/events", conf.Common.PublicBaseURI)
	foundMax := false
	if maxRecords != nil {
		foundMax = true
		url = fmt.Sprintf("%s?max_records=%d", url, *maxRecords)
	}
	if len(slugs) > 0 {
		slugStr := strings.Join(slugs, ",")
		slugParam := fmt.Sprintf("event_type_slugs=%s", slugStr)
		separator := "?"
		if foundMax {
			separator = "&"
		}
		url = fmt.Sprintf("%s%s%s", url, separator, slugParam)
	}
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return nil, 0, err
	}
	data, _ := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("GetEvents error", zap.Any("response", data))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return nil, 0, err
		}
		errResp.Path = url
		return nil, 0, errResp
	}

	// otherwise we found them so unmarshall into class and return true
	var er EventResponse
	if err = json.Unmarshal(data, &er); err != nil {
		return nil, 0, err
	}

	return er.Events, er.LastReadIndex, nil

}

// PUT /api/v1/events/queue/watermark
func SetWatermarkForQueue(ctx context.Context, token string, watermark int64) error {

	defer func() {
		go clientTransport.CloseIdleConnections()
	}()
	conf := config.Current()
	requestID := velacontext.GetContextRequestID(ctx)
	url := fmt.Sprintf("%s/api/v1/events/queue/watermark", conf.Common.PublicBaseURI)
	w := Watermark{
		LastReadIndex: watermark,
	}

	jsonValue, _ := json.Marshal(w)
	request, _ := http.NewRequest("PUT", url, bytes.NewBuffer(jsonValue))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Vela-Request-Id", requestID)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	response, err := apiClient.Do(request)
	if err != nil || response == nil {
		return err
	}
	var dat map[string]interface{}
	data, _ := ioutil.ReadAll(response.Body)
	if err = json.Unmarshal(data, &dat); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		logger := velacontext.GetContextLogger(ctx)
		logger.Info("Setting Watermark error", zap.Any("response", dat))
		var errResp HttpClientError
		if err = json.Unmarshal(data, &errResp); err != nil {
			return err
		}
		if errResp.Fields != nil && len(errResp.Fields) > 0 {
			errMap := ErrorMap{}
			for _, f := range errResp.Fields {
				fn := strings.Split(f.Name, ":")
				errMap.AppendErrorField(fn[len(fn)-1], f.Message)
			}
			return errMap
		}
		errResp.Path = url
		return errResp
	}
	return nil
}

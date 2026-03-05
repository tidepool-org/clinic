package clients

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/tidepool-org/go-common/clients/disc"
	"github.com/tidepool-org/go-common/clients/status"
	"github.com/tidepool-org/go-common/errors"
)

type (
	Pagination struct {
		Page int `json:"page,omitempty"`
		Size int `json:"size,omitempty"`
	}

	DataSourceArray []*DataSource
	DataSetArray    []*DataSet
	//Interface so that we can mock dataClient for tests
	Data interface {
		//userID  -- the Tidepool-assigned userID
		//
		// returns the Data Sources for the user
		ListSources(userID string) (DataSourceArray, error)
		ListSourcesPagination(userID string, p Pagination) (DataSourceArray, error)
		ListSetsPagination(userID string, p Pagination) (DataSetArray, error)
		HasAnyData(userID string) (hasData bool, err error)
	}

	DataClient struct {
		httpClient    *http.Client    // store a reference to the http client so we can reuse it
		hostGetter    disc.HostGetter // The getter that provides the host to talk to for the client
		tokenProvider TokenProvider   // An object that provides tokens for communicating with data
	}

	dataClientBuilder struct {
		httpClient    *http.Client    // store a reference to the http client so we can reuse it
		hostGetter    disc.HostGetter // The getter that provides the host to talk to for the client
		tokenProvider TokenProvider   // An object that provides tokens for communicating with data
	}
)

type DataSource struct {
	ID                *string              `json:"id,omitempty"`
	UserID            *string              `json:"userId,omitempty"`
	ProviderType      *string              `json:"providerType,omitempty"`
	ProviderName      *string              `json:"providerName,omitempty"`
	ProviderSessionID *string              `json:"providerSessionId,omitempty"`
	State             *string              `json:"state,omitempty"`
	Error             *errors.Serializable `json:"error,omitempty"`
	DataSetIDs        *[]string            `json:"dataSetIds,omitempty"`
	EarliestDataTime  *time.Time           `json:"earliestDataTime,omitempty"`
	LatestDataTime    *time.Time           `json:"latestDataTime,omitempty"`
	LastImportTime    *time.Time           `json:"lastImportTime,omitempty"`
	CreatedTime       *time.Time           `json:"createdTime,omitempty"`
	ModifiedTime      *time.Time           `json:"modifiedTime,omitempty"`
	Revision          *int                 `json:"revision,omitempty"`
}

type DataSet struct {
	ByUser              *string    `json:"byUser,omitempty"`
	ClockDriftOffset    *int       `json:"clockDriftOffset,omitempty"`
	ConversionOffset    *int       `json:"conversionOffset,omitempty"`
	CreatedTime         *time.Time `json:"createdTime,omitempty"`
	CreatedUserID       *string    `json:"createdUserId,omitempty"`
	DeletedTime         *time.Time `json:"deletedTime,omitempty"`
	DeletedUserID       *string    `json:"deletedUserId,omitempty"`
	DeviceID            *string    `json:"deviceId,omitempty"`
	DeviceManufacturers *[]string  `json:"deviceManufacturers,omitempty"`
	DeviceModel         *string    `json:"deviceModel,omitempty"`
	DeviceSerialNumber  *string    `json:"deviceSerialNumber,omitempty"`
	DeviceTags          *[]string  `json:"deviceTags,omitempty"`
	DeviceTime          *string    `json:"deviceTime,omitempty"`
	ID                  *string    `json:"id,omitempty"`
	ModifiedTime        *time.Time `json:"modifiedTime,omitempty"`
	ModifiedUserID      *string    `json:"modifiedUserId,omitempty"`
	Time                *time.Time `json:"time,omitempty"`
	TimeProcessing      *string    `json:"timeProcessing,omitempty"`
	TimeZoneName        *string    `json:"timezone,omitempty"`
	TimeZoneOffset      *int       `json:"timezoneOffset,omitempty"`
	UploadID            *string    `json:"uploadId,omitempty"`
}

func NewDataClientBuilder() *dataClientBuilder {
	return &dataClientBuilder{}
}

func (b *dataClientBuilder) WithHttpClient(httpClient *http.Client) *dataClientBuilder {
	b.httpClient = httpClient
	return b
}

func (b *dataClientBuilder) WithHostGetter(hostGetter disc.HostGetter) *dataClientBuilder {
	b.hostGetter = hostGetter
	return b
}

func (b *dataClientBuilder) WithTokenProvider(tokenProvider TokenProvider) *dataClientBuilder {
	b.tokenProvider = tokenProvider
	return b
}

func (b *dataClientBuilder) Build() *DataClient {
	if b.hostGetter == nil {
		panic("dataClient requires a hostGetter to be set")
	}
	if b.tokenProvider == nil {
		panic("dataClient requires a tokenProvider to be set")
	}

	if b.httpClient == nil {
		b.httpClient = http.DefaultClient
	}

	return &DataClient{
		httpClient:    b.httpClient,
		hostGetter:    b.hostGetter,
		tokenProvider: b.tokenProvider,
	}
}

func (client *DataClient) ListSources(userID string) (DataSourceArray, error) {
	return client.ListSourcesPagination(userID, Pagination{Page: 0, Size: 0})
}

func (client *DataClient) ListSourcesPagination(userID string, p Pagination) (DataSourceArray, error) {
	host := client.getHost()
	if host == nil {
		return nil, errors.New("No known data hosts")
	}
	host.Path = path.Join(host.Path, "v1", "users", userID, "data_sources")
	modifyURLWithPagination(host, p)

	req, _ := http.NewRequest(http.MethodGet, host.String(), nil)
	req.Header.Add("x-tidepool-session-token", client.tokenProvider.TokenProvide())

	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		retVal := DataSourceArray{}
		if err := json.NewDecoder(res.Body).Decode(&retVal); err != nil {
			log.Println(err)
			return nil, &status.StatusError{status.NewStatus(500, "ListSources Unable to parse response.")}
		}
		return retVal, nil
	} else if res.StatusCode == http.StatusNotFound {
		return nil, nil
	} else {
		return nil, &status.StatusError{status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL)}
	}
}

func (client *DataClient) ListSetsPagination(userID string, p Pagination) (DataSetArray, error) {
	host := client.getHost()
	if host == nil {
		return nil, errors.New("No known data hosts")
	}
	host.Path = path.Join(host.Path, "v1", "users", userID, "data_sets")
	modifyURLWithPagination(host, p)

	req, _ := http.NewRequest(http.MethodGet, host.String(), nil)
	req.Header.Add("x-tidepool-session-token", client.tokenProvider.TokenProvide())

	res, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		retVal := DataSetArray{}
		if err := json.NewDecoder(res.Body).Decode(&retVal); err != nil {
			log.Println(err)
			return nil, &status.StatusError{status.NewStatus(500, "ListSets Unable to parse response.")}
		}
		return retVal, nil
	} else if res.StatusCode == http.StatusNotFound {
		return nil, nil
	} else {
		return nil, &status.StatusError{status.NewStatusf(res.StatusCode, "Unknown response code from service[%s]", req.URL)}
	}
}

func (client *DataClient) HasAnyData(userID string) (hasData bool, err error) {
	sources, err := client.ListSourcesPagination(userID, Pagination{Page: 0, Size: 1})
	if err != nil {
		return false, err
	}
	if len(sources) > 0 {
		return true, nil
	}
	sets, err := client.ListSetsPagination(userID, Pagination{Page: 0, Size: 1})
	if err != nil {
		return false, err
	}
	return len(sets) > 0, nil
}

func modifyURLWithPagination(u *url.URL, p Pagination) {
	p.Page = max(p.Page, 0)
	p.Size = max(p.Size, 0)
	if p.Page > 0 || p.Size > 0 {
		qsValues := u.Query()
		qsValues.Set("page", fmt.Sprintf("%v", p.Page))
		qsValues.Set("size", fmt.Sprintf("%v", p.Size))
		u.RawQuery = qsValues.Encode()
	}
}

func (client *DataClient) getHost() *url.URL {
	if hostArr := client.hostGetter.HostGet(); len(hostArr) > 0 {
		cpy := new(url.URL)
		*cpy = hostArr[0]
		return cpy
	} else {
		return nil
	}
}

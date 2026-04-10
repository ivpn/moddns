# moddns.PASessionApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_pasession_add_post**](PASessionApi.md#api_v1_pasession_add_post) | **POST** /api/v1/pasession/add | Add pre-auth session
[**api_v1_pasession_rotate_put**](PASessionApi.md#api_v1_pasession_rotate_put) | **PUT** /api/v1/pasession/rotate | Rotate pre-auth session ID


# **api_v1_pasession_add_post**
> Dict[str, object] api_v1_pasession_add_post(body)

Add pre-auth session

Add a pre-auth session to cache (called by preauth service)

### Example


```python
import moddns
from moddns.models.requests_pa_session_req import RequestsPASessionReq
from moddns.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = moddns.Configuration(
    host = "http://localhost"
)


# Enter a context with an instance of the API client
with moddns.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = moddns.PASessionApi(api_client)
    body = moddns.RequestsPASessionReq() # RequestsPASessionReq | Pre-auth session request

    try:
        # Add pre-auth session
        api_response = api_instance.api_v1_pasession_add_post(body)
        print("The response of PASessionApi->api_v1_pasession_add_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling PASessionApi->api_v1_pasession_add_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsPASessionReq**](RequestsPASessionReq.md)| Pre-auth session request | 

### Return type

**Dict[str, object]**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**401** | Unauthorized |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_pasession_rotate_put**
> api_v1_pasession_rotate_put(body)

Rotate pre-auth session ID

Rotate pre-auth session ID and set new ID as cookie

### Example


```python
import moddns
from moddns.models.requests_rotate_pa_session_req import RequestsRotatePASessionReq
from moddns.rest import ApiException
from pprint import pprint

# Defining the host is optional and defaults to http://localhost
# See configuration.py for a list of all supported configuration parameters.
configuration = moddns.Configuration(
    host = "http://localhost"
)


# Enter a context with an instance of the API client
with moddns.ApiClient(configuration) as api_client:
    # Create an instance of the API class
    api_instance = moddns.PASessionApi(api_client)
    body = moddns.RequestsRotatePASessionReq() # RequestsRotatePASessionReq | Rotate pre-auth session request

    try:
        # Rotate pre-auth session ID
        api_instance.api_v1_pasession_rotate_put(body)
    except Exception as e:
        print("Exception when calling PASessionApi->api_v1_pasession_rotate_put: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsRotatePASessionReq**](RequestsRotatePASessionReq.md)| Rotate pre-auth session request | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


# moddns.SessionsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_sessions_delete**](SessionsApi.md#api_v1_sessions_delete) | **DELETE** /api/v1/sessions | Delete all other sessions


# **api_v1_sessions_delete**
> api_v1_sessions_delete()

Delete all other sessions

Delete all sessions for the current account except the current session

### Example


```python
import moddns
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
    api_instance = moddns.SessionsApi(api_client)

    try:
        # Delete all other sessions
        api_instance.api_v1_sessions_delete()
    except Exception as e:
        print("Exception when calling SessionsApi->api_v1_sessions_delete: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


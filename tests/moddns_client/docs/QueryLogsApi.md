# moddns.QueryLogsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_profiles_id_logs_delete**](QueryLogsApi.md#api_v1_profiles_id_logs_delete) | **DELETE** /api/v1/profiles/{id}/logs | Delete profile query logs
[**api_v1_profiles_id_logs_download_get**](QueryLogsApi.md#api_v1_profiles_id_logs_download_get) | **GET** /api/v1/profiles/{id}/logs/download | Download profile query logs
[**api_v1_profiles_id_logs_get**](QueryLogsApi.md#api_v1_profiles_id_logs_get) | **GET** /api/v1/profiles/{id}/logs | Get profile query logs


# **api_v1_profiles_id_logs_delete**
> api_v1_profiles_id_logs_delete(id)

Delete profile query logs

Delete profile query logs

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
    api_instance = moddns.QueryLogsApi(api_client)
    id = 'id_example' # str | Profile ID

    try:
        # Delete profile query logs
        api_instance.api_v1_profiles_id_logs_delete(id)
    except Exception as e:
        print("Exception when calling QueryLogsApi->api_v1_profiles_id_logs_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: */*

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_logs_download_get**
> List[ModelQueryLog] api_v1_profiles_id_logs_download_get(id)

Download profile query logs

Download profile query logs

### Example


```python
import moddns
from moddns.models.model_query_log import ModelQueryLog
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
    api_instance = moddns.QueryLogsApi(api_client)
    id = 'id_example' # str | Profile ID

    try:
        # Download profile query logs
        api_response = api_instance.api_v1_profiles_id_logs_download_get(id)
        print("The response of QueryLogsApi->api_v1_profiles_id_logs_download_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling QueryLogsApi->api_v1_profiles_id_logs_download_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 

### Return type

[**List[ModelQueryLog]**](ModelQueryLog.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_logs_get**
> List[ModelQueryLog] api_v1_profiles_id_logs_get(id, page=page, limit=limit, status=status, timespan=timespan, device_id=device_id, search=search, sort_by=sort_by)

Get profile query logs

Get profile query logs

### Example


```python
import moddns
from moddns.models.model_query_log import ModelQueryLog
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
    api_instance = moddns.QueryLogsApi(api_client)
    id = 'id_example' # str | Profile ID
    page = 1 # int | specify page number (optional) (default to 1)
    limit = 100 # int | specify logs limit by page (optional) (default to 100)
    status = '"all"' # str | specify status for query (optional) (default to '"all"')
    timespan = '"LAST_1_HOUR"' # str | specify timespan for query (optional) (default to '"LAST_1_HOUR"')
    device_id = 'device_id_example' # str | specify device ID for filtering (optional)
    search = 'search_example' # str | substring (case-insensitive) match against stored domain; free-form (short inputs may scan more) (optional)
    sort_by = created # str | field to sort by (optional) (default to created)

    try:
        # Get profile query logs
        api_response = api_instance.api_v1_profiles_id_logs_get(id, page=page, limit=limit, status=status, timespan=timespan, device_id=device_id, search=search, sort_by=sort_by)
        print("The response of QueryLogsApi->api_v1_profiles_id_logs_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling QueryLogsApi->api_v1_profiles_id_logs_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **page** | **int**| specify page number | [optional] [default to 1]
 **limit** | **int**| specify logs limit by page | [optional] [default to 100]
 **status** | **str**| specify status for query | [optional] [default to &#39;&quot;all&quot;&#39;]
 **timespan** | **str**| specify timespan for query | [optional] [default to &#39;&quot;LAST_1_HOUR&quot;&#39;]
 **device_id** | **str**| specify device ID for filtering | [optional] 
 **search** | **str**| substring (case-insensitive) match against stored domain; free-form (short inputs may scan more) | [optional] 
 **sort_by** | **str**| field to sort by | [optional] [default to created]

### Return type

[**List[ModelQueryLog]**](ModelQueryLog.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


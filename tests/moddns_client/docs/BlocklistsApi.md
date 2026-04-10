# moddns.BlocklistsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_blocklists_get**](BlocklistsApi.md#api_v1_blocklists_get) | **GET** /api/v1/blocklists | Get blocklists data


# **api_v1_blocklists_get**
> List[ModelBlocklist] api_v1_blocklists_get(sort_by=sort_by)

Get blocklists data

Get available blocklists data

### Example


```python
import moddns
from moddns.models.model_blocklist import ModelBlocklist
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
    api_instance = moddns.BlocklistsApi(api_client)
    sort_by = updated # str | field to sort by (optional) (default to updated)

    try:
        # Get blocklists data
        api_response = api_instance.api_v1_blocklists_get(sort_by=sort_by)
        print("The response of BlocklistsApi->api_v1_blocklists_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling BlocklistsApi->api_v1_blocklists_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **sort_by** | **str**| field to sort by | [optional] [default to updated]

### Return type

[**List[ModelBlocklist]**](ModelBlocklist.md)

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


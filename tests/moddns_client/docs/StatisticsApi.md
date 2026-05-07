# moddns.StatisticsApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_profiles_id_statistics_get**](StatisticsApi.md#api_v1_profiles_id_statistics_get) | **GET** /api/v1/profiles/{id}/statistics | Get statistics data for a profile


# **api_v1_profiles_id_statistics_get**
> List[ModelStatisticsAggregated] api_v1_profiles_id_statistics_get(id, timespan=timespan)

Get statistics data for a profile

Get statistics data for a profile

### Example


```python
import moddns
from moddns.models.model_statistics_aggregated import ModelStatisticsAggregated
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
    api_instance = moddns.StatisticsApi(api_client)
    id = 'id_example' # str | Profile ID
    timespan = '"LAST_MONTH"' # str | specify timespan for query (optional) (default to '"LAST_MONTH"')

    try:
        # Get statistics data for a profile
        api_response = api_instance.api_v1_profiles_id_statistics_get(id, timespan=timespan)
        print("The response of StatisticsApi->api_v1_profiles_id_statistics_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling StatisticsApi->api_v1_profiles_id_statistics_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **timespan** | **str**| specify timespan for query | [optional] [default to &#39;&quot;LAST_MONTH&quot;&#39;]

### Return type

[**List[ModelStatisticsAggregated]**](ModelStatisticsAggregated.md)

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


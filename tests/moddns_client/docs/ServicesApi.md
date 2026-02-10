# moddns.ServicesApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_services_get**](ServicesApi.md#api_v1_services_get) | **GET** /api/v1/services | Get services catalog


# **api_v1_services_get**
> ServicescatalogCatalog api_v1_services_get()

Get services catalog

Get available ASN-based services presets

### Example


```python
import moddns
from moddns.models.servicescatalog_catalog import ServicescatalogCatalog
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
    api_instance = moddns.ServicesApi(api_client)

    try:
        # Get services catalog
        api_response = api_instance.api_v1_services_get()
        print("The response of ServicesApi->api_v1_services_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ServicesApi->api_v1_services_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ServicescatalogCatalog**](ServicescatalogCatalog.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


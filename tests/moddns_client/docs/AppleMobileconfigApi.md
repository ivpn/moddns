# moddns.AppleMobileconfigApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_mobileconfig_post**](AppleMobileconfigApi.md#api_v1_mobileconfig_post) | **POST** /api/v1/mobileconfig | Generate configuration profile for Apple devices
[**api_v1_mobileconfig_short_post**](AppleMobileconfigApi.md#api_v1_mobileconfig_short_post) | **POST** /api/v1/mobileconfig/short | Generate short link for configuration profile (Apple devices)
[**api_v1_short_code_get**](AppleMobileconfigApi.md#api_v1_short_code_get) | **GET** /api/v1/short/{code} | Download configuration profile for Apple devices from short link


# **api_v1_mobileconfig_post**
> str api_v1_mobileconfig_post(body)

Generate configuration profile for Apple devices

Generate configuration profile for Apple devices

### Example


```python
import moddns
from moddns.models.requests_mobile_config_req import RequestsMobileConfigReq
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
    api_instance = moddns.AppleMobileconfigApi(api_client)
    body = moddns.RequestsMobileConfigReq() # RequestsMobileConfigReq | Generate .mobileconfig request

    try:
        # Generate configuration profile for Apple devices
        api_response = api_instance.api_v1_mobileconfig_post(body)
        print("The response of AppleMobileconfigApi->api_v1_mobileconfig_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AppleMobileconfigApi->api_v1_mobileconfig_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsMobileConfigReq**](RequestsMobileConfigReq.md)| Generate .mobileconfig request | 

### Return type

**str**

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_mobileconfig_short_post**
> ResponsesShortLinkResponse api_v1_mobileconfig_short_post(body)

Generate short link for configuration profile (Apple devices)

Generate short link for configuration profile (Apple devices)

### Example


```python
import moddns
from moddns.models.requests_mobile_config_req import RequestsMobileConfigReq
from moddns.models.responses_short_link_response import ResponsesShortLinkResponse
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
    api_instance = moddns.AppleMobileconfigApi(api_client)
    body = moddns.RequestsMobileConfigReq() # RequestsMobileConfigReq | Generate .mobileconfig request

    try:
        # Generate short link for configuration profile (Apple devices)
        api_response = api_instance.api_v1_mobileconfig_short_post(body)
        print("The response of AppleMobileconfigApi->api_v1_mobileconfig_short_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AppleMobileconfigApi->api_v1_mobileconfig_short_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsMobileConfigReq**](RequestsMobileConfigReq.md)| Generate .mobileconfig request | 

### Return type

[**ResponsesShortLinkResponse**](ResponsesShortLinkResponse.md)

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_short_code_get**
> str api_v1_short_code_get(code)

Download configuration profile for Apple devices from short link

Download configuration profile for Apple devices from short link

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
    api_instance = moddns.AppleMobileconfigApi(api_client)
    code = 'code_example' # str | short code

    try:
        # Download configuration profile for Apple devices from short link
        api_response = api_instance.api_v1_short_code_get(code)
        print("The response of AppleMobileconfigApi->api_v1_short_code_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AppleMobileconfigApi->api_v1_short_code_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **code** | **str**| short code | 

### Return type

**str**

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


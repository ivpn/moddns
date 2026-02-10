# moddns.SubscriptionApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_sub_get**](SubscriptionApi.md#api_v1_sub_get) | **GET** /api/v1/sub | Get subscription data
[**api_v1_subscription_add_post**](SubscriptionApi.md#api_v1_subscription_add_post) | **POST** /api/v1/subscription/add | Add subscription


# **api_v1_sub_get**
> ModelSubscription api_v1_sub_get()

Get subscription data

Get subscription data for the authenticated account

### Example


```python
import moddns
from moddns.models.model_subscription import ModelSubscription
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
    api_instance = moddns.SubscriptionApi(api_client)

    try:
        # Get subscription data
        api_response = api_instance.api_v1_sub_get()
        print("The response of SubscriptionApi->api_v1_sub_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SubscriptionApi->api_v1_sub_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ModelSubscription**](ModelSubscription.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**200** | OK |  -  |
**401** | Unauthorized |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_subscription_add_post**
> Dict[str, object] api_v1_subscription_add_post(body)

Add subscription

Add subscription and cache its presence

### Example


```python
import moddns
from moddns.models.requests_subscription_req import RequestsSubscriptionReq
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
    api_instance = moddns.SubscriptionApi(api_client)
    body = moddns.RequestsSubscriptionReq() # RequestsSubscriptionReq | Subscription request

    try:
        # Add subscription
        api_response = api_instance.api_v1_subscription_add_post(body)
        print("The response of SubscriptionApi->api_v1_subscription_add_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling SubscriptionApi->api_v1_subscription_add_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsSubscriptionReq**](RequestsSubscriptionReq.md)| Subscription request | 

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
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


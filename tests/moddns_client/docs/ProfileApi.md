# moddns.ProfileApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_profiles_get**](ProfileApi.md#api_v1_profiles_get) | **GET** /api/v1/profiles | Get profiles data
[**api_v1_profiles_id_blocklists_delete**](ProfileApi.md#api_v1_profiles_id_blocklists_delete) | **DELETE** /api/v1/profiles/{id}/blocklists | Disable blocklists
[**api_v1_profiles_id_blocklists_post**](ProfileApi.md#api_v1_profiles_id_blocklists_post) | **POST** /api/v1/profiles/{id}/blocklists | Enable blocklists
[**api_v1_profiles_id_custom_rules_batch_post**](ProfileApi.md#api_v1_profiles_id_custom_rules_batch_post) | **POST** /api/v1/profiles/{id}/custom_rules/batch | Create profile custom rules (batch)
[**api_v1_profiles_id_custom_rules_custom_rule_id_delete**](ProfileApi.md#api_v1_profiles_id_custom_rules_custom_rule_id_delete) | **DELETE** /api/v1/profiles/{id}/custom_rules/{custom_rule_id} | Delete profile custom rule
[**api_v1_profiles_id_custom_rules_post**](ProfileApi.md#api_v1_profiles_id_custom_rules_post) | **POST** /api/v1/profiles/{id}/custom_rules | Create profile custom rule
[**api_v1_profiles_id_delete**](ProfileApi.md#api_v1_profiles_id_delete) | **DELETE** /api/v1/profiles/{id} | Delete profile
[**api_v1_profiles_id_get**](ProfileApi.md#api_v1_profiles_id_get) | **GET** /api/v1/profiles/{id} | Get profile data
[**api_v1_profiles_id_patch**](ProfileApi.md#api_v1_profiles_id_patch) | **PATCH** /api/v1/profiles/{id} | Update profile
[**api_v1_profiles_id_services_delete**](ProfileApi.md#api_v1_profiles_id_services_delete) | **DELETE** /api/v1/profiles/{id}/services | Disable services
[**api_v1_profiles_id_services_post**](ProfileApi.md#api_v1_profiles_id_services_post) | **POST** /api/v1/profiles/{id}/services | Enable services
[**api_v1_profiles_post**](ProfileApi.md#api_v1_profiles_post) | **POST** /api/v1/profiles | Create profile


# **api_v1_profiles_get**
> List[ModelProfile] api_v1_profiles_get()

Get profiles data

Get profiles data

### Example


```python
import moddns
from moddns.models.model_profile import ModelProfile
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
    api_instance = moddns.ProfileApi(api_client)

    try:
        # Get profiles data
        api_response = api_instance.api_v1_profiles_get()
        print("The response of ProfileApi->api_v1_profiles_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[ModelProfile]**](ModelProfile.md)

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

# **api_v1_profiles_id_blocklists_delete**
> api_v1_profiles_id_blocklists_delete(id, blocklist_ids)

Disable blocklists

Disable blocklists for a profile

### Example


```python
import moddns
from moddns.models.api_blocklists_updates import ApiBlocklistsUpdates
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    blocklist_ids = moddns.ApiBlocklistsUpdates() # ApiBlocklistsUpdates | Blocklists to disable

    try:
        # Disable blocklists
        api_instance.api_v1_profiles_id_blocklists_delete(id, blocklist_ids)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_blocklists_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **blocklist_ids** | [**ApiBlocklistsUpdates**](ApiBlocklistsUpdates.md)| Blocklists to disable | 

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_blocklists_post**
> api_v1_profiles_id_blocklists_post(id, blocklist_ids)

Enable blocklists

Enable blocklists for a profile

### Example


```python
import moddns
from moddns.models.api_blocklists_updates import ApiBlocklistsUpdates
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    blocklist_ids = moddns.ApiBlocklistsUpdates() # ApiBlocklistsUpdates | Blocklists to disable

    try:
        # Enable blocklists
        api_instance.api_v1_profiles_id_blocklists_post(id, blocklist_ids)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_blocklists_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **blocklist_ids** | [**ApiBlocklistsUpdates**](ApiBlocklistsUpdates.md)| Blocklists to disable | 

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_custom_rules_batch_post**
> ResponsesCreateProfileCustomRulesBatchResponse api_v1_profiles_id_custom_rules_batch_post(id, body)

Create profile custom rules (batch)

Create up to 20 custom rules for a profile in a single request

### Example


```python
import moddns
from moddns.models.requests_create_profile_custom_rules_batch_body import RequestsCreateProfileCustomRulesBatchBody
from moddns.models.responses_create_profile_custom_rules_batch_response import ResponsesCreateProfileCustomRulesBatchResponse
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    body = moddns.RequestsCreateProfileCustomRulesBatchBody() # RequestsCreateProfileCustomRulesBatchBody | Create custom rules batch request

    try:
        # Create profile custom rules (batch)
        api_response = api_instance.api_v1_profiles_id_custom_rules_batch_post(id, body)
        print("The response of ProfileApi->api_v1_profiles_id_custom_rules_batch_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_custom_rules_batch_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **body** | [**RequestsCreateProfileCustomRulesBatchBody**](RequestsCreateProfileCustomRulesBatchBody.md)| Create custom rules batch request | 

### Return type

[**ResponsesCreateProfileCustomRulesBatchResponse**](ResponsesCreateProfileCustomRulesBatchResponse.md)

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

# **api_v1_profiles_id_custom_rules_custom_rule_id_delete**
> api_v1_profiles_id_custom_rules_custom_rule_id_delete(id, custom_rule_id)

Delete profile custom rule

Delete profile custom rule

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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    custom_rule_id = 'custom_rule_id_example' # str | Custom rule ID

    try:
        # Delete profile custom rule
        api_instance.api_v1_profiles_id_custom_rules_custom_rule_id_delete(id, custom_rule_id)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_custom_rules_custom_rule_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **custom_rule_id** | **str**| Custom rule ID | 

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
**200** | OK |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_custom_rules_post**
> api_v1_profiles_id_custom_rules_post(id, body)

Create profile custom rule

Create profile custom rule

### Example


```python
import moddns
from moddns.models.requests_create_profile_custom_rule_body import RequestsCreateProfileCustomRuleBody
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    body = moddns.RequestsCreateProfileCustomRuleBody() # RequestsCreateProfileCustomRuleBody | Create custom rule request

    try:
        # Create profile custom rule
        api_instance.api_v1_profiles_id_custom_rules_post(id, body)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_custom_rules_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **body** | [**RequestsCreateProfileCustomRuleBody**](RequestsCreateProfileCustomRuleBody.md)| Create custom rule request | 

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
**201** | Created |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_delete**
> api_v1_profiles_id_delete(id)

Delete profile

Delete profile

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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID

    try:
        # Delete profile
        api_instance.api_v1_profiles_id_delete(id)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_delete: %s\n" % e)
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
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_get**
> ModelProfile api_v1_profiles_id_get(id)

Get profile data

Get profile data

### Example


```python
import moddns
from moddns.models.model_profile import ModelProfile
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID

    try:
        # Get profile data
        api_response = api_instance.api_v1_profiles_id_get(id)
        print("The response of ProfileApi->api_v1_profiles_id_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_get: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 

### Return type

[**ModelProfile**](ModelProfile.md)

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

# **api_v1_profiles_id_patch**
> ModelProfile api_v1_profiles_id_patch(id, body)

Update profile

Update profile

### Example


```python
import moddns
from moddns.models.model_profile import ModelProfile
from moddns.models.requests_profile_updates import RequestsProfileUpdates
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    body = moddns.RequestsProfileUpdates() # RequestsProfileUpdates | Update profile

    try:
        # Update profile
        api_response = api_instance.api_v1_profiles_id_patch(id, body)
        print("The response of ProfileApi->api_v1_profiles_id_patch:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_patch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **body** | [**RequestsProfileUpdates**](RequestsProfileUpdates.md)| Update profile | 

### Return type

[**ModelProfile**](ModelProfile.md)

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

# **api_v1_profiles_id_services_delete**
> api_v1_profiles_id_services_delete(id, service_ids)

Disable services

Disable services for a profile (removes from privacy.services)

### Example


```python
import moddns
from moddns.models.api_services_updates import ApiServicesUpdates
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    service_ids = moddns.ApiServicesUpdates() # ApiServicesUpdates | Services to disable

    try:
        # Disable services
        api_instance.api_v1_profiles_id_services_delete(id, service_ids)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_services_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **service_ids** | [**ApiServicesUpdates**](ApiServicesUpdates.md)| Services to disable | 

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_id_services_post**
> api_v1_profiles_id_services_post(id, service_ids)

Enable services

Enable services for a profile (adds to privacy.services)

### Example


```python
import moddns
from moddns.models.api_services_updates import ApiServicesUpdates
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
    api_instance = moddns.ProfileApi(api_client)
    id = 'id_example' # str | Profile ID
    service_ids = moddns.ApiServicesUpdates() # ApiServicesUpdates | Services to enable

    try:
        # Enable services
        api_instance.api_v1_profiles_id_services_post(id, service_ids)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_id_services_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Profile ID | 
 **service_ids** | [**ApiServicesUpdates**](ApiServicesUpdates.md)| Services to enable | 

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
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_profiles_post**
> ModelProfile api_v1_profiles_post(body)

Create profile

Create profile

### Example


```python
import moddns
from moddns.models.api_create_profile_body import ApiCreateProfileBody
from moddns.models.model_profile import ModelProfile
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
    api_instance = moddns.ProfileApi(api_client)
    body = moddns.ApiCreateProfileBody() # ApiCreateProfileBody | Create profile request

    try:
        # Create profile
        api_response = api_instance.api_v1_profiles_post(body)
        print("The response of ProfileApi->api_v1_profiles_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling ProfileApi->api_v1_profiles_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**ApiCreateProfileBody**](ApiCreateProfileBody.md)| Create profile request | 

### Return type

[**ModelProfile**](ModelProfile.md)

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


# moddns.AccountApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_accounts_current_delete**](AccountApi.md#api_v1_accounts_current_delete) | **DELETE** /api/v1/accounts/current | Delete account
[**api_v1_accounts_current_deletion_code_post**](AccountApi.md#api_v1_accounts_current_deletion_code_post) | **POST** /api/v1/accounts/current/deletion-code | Generate deletion code
[**api_v1_accounts_current_get**](AccountApi.md#api_v1_accounts_current_get) | **GET** /api/v1/accounts/current | Get account data
[**api_v1_accounts_mfa_totp_disable_post**](AccountApi.md#api_v1_accounts_mfa_totp_disable_post) | **POST** /api/v1/accounts/mfa/totp/disable | Disable TOTP
[**api_v1_accounts_mfa_totp_enable_confirm_post**](AccountApi.md#api_v1_accounts_mfa_totp_enable_confirm_post) | **POST** /api/v1/accounts/mfa/totp/enable/confirm | Confirm TOTP
[**api_v1_accounts_mfa_totp_enable_post**](AccountApi.md#api_v1_accounts_mfa_totp_enable_post) | **POST** /api/v1/accounts/mfa/totp/enable | Enable TOTP
[**api_v1_accounts_patch**](AccountApi.md#api_v1_accounts_patch) | **PATCH** /api/v1/accounts | Update account
[**api_v1_accounts_post**](AccountApi.md#api_v1_accounts_post) | **POST** /api/v1/accounts | Register account
[**api_v1_accounts_reset_password_post**](AccountApi.md#api_v1_accounts_reset_password_post) | **POST** /api/v1/accounts/reset-password | Send reset password email


# **api_v1_accounts_current_delete**
> api_v1_accounts_current_delete(body)

Delete account

Delete account with deletion code

### Example


```python
import moddns
from moddns.models.requests_account_deletion_request import RequestsAccountDeletionRequest
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.RequestsAccountDeletionRequest() # RequestsAccountDeletionRequest | Account deletion request

    try:
        # Delete account
        api_instance.api_v1_accounts_current_delete(body)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_current_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsAccountDeletionRequest**](RequestsAccountDeletionRequest.md)| Account deletion request | 

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
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_accounts_current_deletion_code_post**
> ResponsesDeletionCodeResponse api_v1_accounts_current_deletion_code_post()

Generate deletion code

Generate a deletion code for account deletion

### Example


```python
import moddns
from moddns.models.responses_deletion_code_response import ResponsesDeletionCodeResponse
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
    api_instance = moddns.AccountApi(api_client)

    try:
        # Generate deletion code
        api_response = api_instance.api_v1_accounts_current_deletion_code_post()
        print("The response of AccountApi->api_v1_accounts_current_deletion_code_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_current_deletion_code_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ResponsesDeletionCodeResponse**](ResponsesDeletionCodeResponse.md)

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

# **api_v1_accounts_current_get**
> ModelAccount api_v1_accounts_current_get()

Get account data

Get account data

### Example


```python
import moddns
from moddns.models.model_account import ModelAccount
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
    api_instance = moddns.AccountApi(api_client)

    try:
        # Get account data
        api_response = api_instance.api_v1_accounts_current_get()
        print("The response of AccountApi->api_v1_accounts_current_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_current_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ModelAccount**](ModelAccount.md)

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

# **api_v1_accounts_mfa_totp_disable_post**
> ModelAccount api_v1_accounts_mfa_totp_disable_post(body)

Disable TOTP

Disable TOTP

### Example


```python
import moddns
from moddns.models.model_account import ModelAccount
from moddns.models.requests_totp_req import RequestsTotpReq
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.RequestsTotpReq() # RequestsTotpReq | Disable TOTP request

    try:
        # Disable TOTP
        api_response = api_instance.api_v1_accounts_mfa_totp_disable_post(body)
        print("The response of AccountApi->api_v1_accounts_mfa_totp_disable_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_mfa_totp_disable_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsTotpReq**](RequestsTotpReq.md)| Disable TOTP request | 

### Return type

[**ModelAccount**](ModelAccount.md)

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

# **api_v1_accounts_mfa_totp_enable_confirm_post**
> ModelTOTPBackup api_v1_accounts_mfa_totp_enable_confirm_post(body)

Confirm TOTP

Confirm TOTP

### Example


```python
import moddns
from moddns.models.model_totp_backup import ModelTOTPBackup
from moddns.models.requests_totp_req import RequestsTotpReq
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.RequestsTotpReq() # RequestsTotpReq | Confirm TOTP request

    try:
        # Confirm TOTP
        api_response = api_instance.api_v1_accounts_mfa_totp_enable_confirm_post(body)
        print("The response of AccountApi->api_v1_accounts_mfa_totp_enable_confirm_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_mfa_totp_enable_confirm_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsTotpReq**](RequestsTotpReq.md)| Confirm TOTP request | 

### Return type

[**ModelTOTPBackup**](ModelTOTPBackup.md)

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

# **api_v1_accounts_mfa_totp_enable_post**
> ModelTOTPNew api_v1_accounts_mfa_totp_enable_post()

Enable TOTP

Enable TOTP

### Example


```python
import moddns
from moddns.models.model_totp_new import ModelTOTPNew
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
    api_instance = moddns.AccountApi(api_client)

    try:
        # Enable TOTP
        api_response = api_instance.api_v1_accounts_mfa_totp_enable_post()
        print("The response of AccountApi->api_v1_accounts_mfa_totp_enable_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_mfa_totp_enable_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ModelTOTPNew**](ModelTOTPNew.md)

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

# **api_v1_accounts_patch**
> api_v1_accounts_patch(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods)

Update account

Update account

### Example


```python
import moddns
from moddns.models.requests_account_updates import RequestsAccountUpdates
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.RequestsAccountUpdates() # RequestsAccountUpdates | Update account request
    x_mfa_code = 'x_mfa_code_example' # str | MFA OTP code (optional)
    x_mfa_methods = ['x_mfa_methods_example'] # List[str] | MFA methods (optional)

    try:
        # Update account
        api_instance.api_v1_accounts_patch(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_patch: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsAccountUpdates**](RequestsAccountUpdates.md)| Update account request | 
 **x_mfa_code** | **str**| MFA OTP code | [optional] 
 **x_mfa_methods** | [**List[str]**](str.md)| MFA methods | [optional] 

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
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_accounts_post**
> ResponsesRegistrationSuccessResponse api_v1_accounts_post(body)

Register account

Register account

### Example


```python
import moddns
from moddns.models.api_register_account_body import ApiRegisterAccountBody
from moddns.models.responses_registration_success_response import ResponsesRegistrationSuccessResponse
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.ApiRegisterAccountBody() # ApiRegisterAccountBody | Account request

    try:
        # Register account
        api_response = api_instance.api_v1_accounts_post(body)
        print("The response of AccountApi->api_v1_accounts_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**ApiRegisterAccountBody**](ApiRegisterAccountBody.md)| Account request | 

### Return type

[**ResponsesRegistrationSuccessResponse**](ResponsesRegistrationSuccessResponse.md)

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

# **api_v1_accounts_reset_password_post**
> api_v1_accounts_reset_password_post(body)

Send reset password email

Send reset password email

### Example


```python
import moddns
from moddns.models.requests_reset_password_body import RequestsResetPasswordBody
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
    api_instance = moddns.AccountApi(api_client)
    body = moddns.RequestsResetPasswordBody() # RequestsResetPasswordBody | Send reset password email request

    try:
        # Send reset password email
        api_instance.api_v1_accounts_reset_password_post(body)
    except Exception as e:
        print("Exception when calling AccountApi->api_v1_accounts_reset_password_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsResetPasswordBody**](RequestsResetPasswordBody.md)| Send reset password email request | 

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
**204** | No Content |  -  |
**400** | Bad Request |  -  |
**404** | Not Found |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


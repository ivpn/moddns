# moddns.AuthenticationApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_accounts_logout_post**](AuthenticationApi.md#api_v1_accounts_logout_post) | **POST** /api/v1/accounts/logout | Logout
[**api_v1_login_post**](AuthenticationApi.md#api_v1_login_post) | **POST** /api/v1/login | Login
[**api_v1_webauthn_login_begin_post**](AuthenticationApi.md#api_v1_webauthn_login_begin_post) | **POST** /api/v1/webauthn/login/begin | Begin passkey login
[**api_v1_webauthn_login_finish_post**](AuthenticationApi.md#api_v1_webauthn_login_finish_post) | **POST** /api/v1/webauthn/login/finish | Finish passkey login
[**api_v1_webauthn_passkey_add_begin_post**](AuthenticationApi.md#api_v1_webauthn_passkey_add_begin_post) | **POST** /api/v1/webauthn/passkey/add/begin | Add new passkey
[**api_v1_webauthn_passkey_add_finish_post**](AuthenticationApi.md#api_v1_webauthn_passkey_add_finish_post) | **POST** /api/v1/webauthn/passkey/add/finish | Complete adding new passkey
[**api_v1_webauthn_passkey_id_delete**](AuthenticationApi.md#api_v1_webauthn_passkey_id_delete) | **DELETE** /api/v1/webauthn/passkey/{id} | Delete passkey
[**api_v1_webauthn_passkey_reauth_begin_post**](AuthenticationApi.md#api_v1_webauthn_passkey_reauth_begin_post) | **POST** /api/v1/webauthn/passkey/reauth/begin | Begin reauthentication via passkey
[**api_v1_webauthn_passkey_reauth_finish_post**](AuthenticationApi.md#api_v1_webauthn_passkey_reauth_finish_post) | **POST** /api/v1/webauthn/passkey/reauth/finish | Finish reauthentication via passkey
[**api_v1_webauthn_passkeys_get**](AuthenticationApi.md#api_v1_webauthn_passkeys_get) | **GET** /api/v1/webauthn/passkeys | Get user passkeys
[**api_v1_webauthn_register_begin_post**](AuthenticationApi.md#api_v1_webauthn_register_begin_post) | **POST** /api/v1/webauthn/register/begin | Begin passkey registration
[**api_v1_webauthn_register_finish_post**](AuthenticationApi.md#api_v1_webauthn_register_finish_post) | **POST** /api/v1/webauthn/register/finish | Finish passkey registration


# **api_v1_accounts_logout_post**
> api_v1_accounts_logout_post()

Logout

Logout endpoint

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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Logout
        api_instance.api_v1_accounts_logout_post()
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_accounts_logout_post: %s\n" % e)
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
**200** | OK |  -  |
**400** | Bad Request |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_login_post**
> api_v1_login_post(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods, x_sessions_remove=x_sessions_remove)

Login

Login endpoint

### Example


```python
import moddns
from moddns.models.requests_login_body import RequestsLoginBody
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
    api_instance = moddns.AuthenticationApi(api_client)
    body = moddns.RequestsLoginBody() # RequestsLoginBody | Login request
    x_mfa_code = 'x_mfa_code_example' # str | MFA OTP code (optional)
    x_mfa_methods = ['x_mfa_methods_example'] # List[str] | MFA methods (optional)
    x_sessions_remove = 'x_sessions_remove_example' # str | Remove all active sessions before logging in (optional)

    try:
        # Login
        api_instance.api_v1_login_post(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods, x_sessions_remove=x_sessions_remove)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_login_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsLoginBody**](RequestsLoginBody.md)| Login request | 
 **x_mfa_code** | **str**| MFA OTP code | [optional] 
 **x_mfa_methods** | [**List[str]**](str.md)| MFA methods | [optional] 
 **x_sessions_remove** | **str**| Remove all active sessions before logging in | [optional] 

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
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_login_begin_post**
> ProtocolCredentialCreation api_v1_webauthn_login_begin_post(body)

Begin passkey login

Start WebAuthn login process

### Example


```python
import moddns
from moddns.models.api_web_authn_login_begin_request import ApiWebAuthnLoginBeginRequest
from moddns.models.protocol_credential_creation import ProtocolCredentialCreation
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
    api_instance = moddns.AuthenticationApi(api_client)
    body = moddns.ApiWebAuthnLoginBeginRequest() # ApiWebAuthnLoginBeginRequest | Login request

    try:
        # Begin passkey login
        api_response = api_instance.api_v1_webauthn_login_begin_post(body)
        print("The response of AuthenticationApi->api_v1_webauthn_login_begin_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_login_begin_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**ApiWebAuthnLoginBeginRequest**](ApiWebAuthnLoginBeginRequest.md)| Login request | 

### Return type

[**ProtocolCredentialCreation**](ProtocolCredentialCreation.md)

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

# **api_v1_webauthn_login_finish_post**
> api_v1_webauthn_login_finish_post(x_sessions_remove=x_sessions_remove)

Finish passkey login

Complete WebAuthn login process

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
    api_instance = moddns.AuthenticationApi(api_client)
    x_sessions_remove = 'x_sessions_remove_example' # str | Remove all other active sessions during login (optional)

    try:
        # Finish passkey login
        api_instance.api_v1_webauthn_login_finish_post(x_sessions_remove=x_sessions_remove)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_login_finish_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **x_sessions_remove** | **str**| Remove all other active sessions during login | [optional] 

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
**201** | Login completed successfully |  -  |
**400** | Bad Request |  -  |
**429** | Session limit reached |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkey_add_begin_post**
> object api_v1_webauthn_passkey_add_begin_post()

Add new passkey

Add a new passkey to authenticated account

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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Add new passkey
        api_response = api_instance.api_v1_webauthn_passkey_add_begin_post()
        print("The response of AuthenticationApi->api_v1_webauthn_passkey_add_begin_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkey_add_begin_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

**object**

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkey_add_finish_post**
> api_v1_webauthn_passkey_add_finish_post()

Complete adding new passkey

Complete adding a new passkey to authenticated account

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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Complete adding new passkey
        api_instance.api_v1_webauthn_passkey_add_finish_post()
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkey_add_finish_post: %s\n" % e)
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
**201** | Passkey addition completed successfully |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkey_id_delete**
> api_v1_webauthn_passkey_id_delete(id)

Delete passkey

Delete a specific passkey

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
    api_instance = moddns.AuthenticationApi(api_client)
    id = 'id_example' # str | Credential ID

    try:
        # Delete passkey
        api_instance.api_v1_webauthn_passkey_id_delete(id)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkey_id_delete: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **id** | **str**| Credential ID | 

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkey_reauth_begin_post**
> ProtocolCredentialAssertion api_v1_webauthn_passkey_reauth_begin_post(body)

Begin reauthentication via passkey

Initiate a WebAuthn assertion to elevate privileges (e.g., email change)

### Example


```python
import moddns
from moddns.models.protocol_credential_assertion import ProtocolCredentialAssertion
from moddns.models.requests_web_authn_reauth_begin_request import RequestsWebAuthnReauthBeginRequest
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
    api_instance = moddns.AuthenticationApi(api_client)
    body = moddns.RequestsWebAuthnReauthBeginRequest() # RequestsWebAuthnReauthBeginRequest | Reauth begin request

    try:
        # Begin reauthentication via passkey
        api_response = api_instance.api_v1_webauthn_passkey_reauth_begin_post(body)
        print("The response of AuthenticationApi->api_v1_webauthn_passkey_reauth_begin_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkey_reauth_begin_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsWebAuthnReauthBeginRequest**](RequestsWebAuthnReauthBeginRequest.md)| Reauth begin request | 

### Return type

[**ProtocolCredentialAssertion**](ProtocolCredentialAssertion.md)

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
**429** | Rate limited |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkey_reauth_finish_post**
> ResponsesWebAuthnReauthFinishResponse api_v1_webauthn_passkey_reauth_finish_post()

Finish reauthentication via passkey

Complete WebAuthn assertion and issue a short-lived reauth token

### Example


```python
import moddns
from moddns.models.responses_web_authn_reauth_finish_response import ResponsesWebAuthnReauthFinishResponse
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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Finish reauthentication via passkey
        api_response = api_instance.api_v1_webauthn_passkey_reauth_finish_post()
        print("The response of AuthenticationApi->api_v1_webauthn_passkey_reauth_finish_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkey_reauth_finish_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**ResponsesWebAuthnReauthFinishResponse**](ResponsesWebAuthnReauthFinishResponse.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

### HTTP response details

| Status code | Description | Response headers |
|-------------|-------------|------------------|
**201** | Created |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_passkeys_get**
> List[ModelCredential] api_v1_webauthn_passkeys_get()

Get user passkeys

Get list of passkeys for authenticated user

### Example


```python
import moddns
from moddns.models.model_credential import ModelCredential
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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Get user passkeys
        api_response = api_instance.api_v1_webauthn_passkeys_get()
        print("The response of AuthenticationApi->api_v1_webauthn_passkeys_get:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_passkeys_get: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

### Return type

[**List[ModelCredential]**](ModelCredential.md)

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

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_webauthn_register_begin_post**
> ProtocolCredentialCreation api_v1_webauthn_register_begin_post(body)

Begin passkey registration

Start WebAuthn registration process for new passkey

### Example


```python
import moddns
from moddns.models.api_web_authn_register_begin_request import ApiWebAuthnRegisterBeginRequest
from moddns.models.protocol_credential_creation import ProtocolCredentialCreation
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
    api_instance = moddns.AuthenticationApi(api_client)
    body = moddns.ApiWebAuthnRegisterBeginRequest() # ApiWebAuthnRegisterBeginRequest | Registration request

    try:
        # Begin passkey registration
        api_response = api_instance.api_v1_webauthn_register_begin_post(body)
        print("The response of AuthenticationApi->api_v1_webauthn_register_begin_post:\n")
        pprint(api_response)
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_register_begin_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**ApiWebAuthnRegisterBeginRequest**](ApiWebAuthnRegisterBeginRequest.md)| Registration request | 

### Return type

[**ProtocolCredentialCreation**](ProtocolCredentialCreation.md)

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

# **api_v1_webauthn_register_finish_post**
> api_v1_webauthn_register_finish_post()

Finish passkey registration

Complete WebAuthn registration process

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
    api_instance = moddns.AuthenticationApi(api_client)

    try:
        # Finish passkey registration
        api_instance.api_v1_webauthn_register_finish_post()
    except Exception as e:
        print("Exception when calling AuthenticationApi->api_v1_webauthn_register_finish_post: %s\n" % e)
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
**201** | Registration completed successfully |  -  |
**400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


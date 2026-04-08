# moddns.VerificationApi

All URIs are relative to *http://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**api_v1_verify_email_otp_confirm_post**](VerificationApi.md#api_v1_verify_email_otp_confirm_post) | **POST** /api/v1/verify/email/otp/confirm | Confirm email verification OTP
[**api_v1_verify_email_otp_request_post**](VerificationApi.md#api_v1_verify_email_otp_request_post) | **POST** /api/v1/verify/email/otp/request | Request email verification OTP
[**api_v1_verify_reset_password_post**](VerificationApi.md#api_v1_verify_reset_password_post) | **POST** /api/v1/verify/reset-password | Confirm password reset


# **api_v1_verify_email_otp_confirm_post**
> api_v1_verify_email_otp_confirm_post(body)

Confirm email verification OTP

Verifies the 6-digit OTP provided by the authenticated user

### Example


```python
import moddns
from moddns.models.api_verify_email_otp_body import ApiVerifyEmailOTPBody
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
    api_instance = moddns.VerificationApi(api_client)
    body = moddns.ApiVerifyEmailOTPBody() # ApiVerifyEmailOTPBody | OTP verification request

    try:
        # Confirm email verification OTP
        api_instance.api_v1_verify_email_otp_confirm_post(body)
    except Exception as e:
        print("Exception when calling VerificationApi->api_v1_verify_email_otp_confirm_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**ApiVerifyEmailOTPBody**](ApiVerifyEmailOTPBody.md)| OTP verification request | 

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
**401** | Unauthorized |  -  |
**410** | Gone |  -  |
**422** | Unprocessable Entity |  -  |
**429** | Too Many Requests |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_verify_email_otp_request_post**
> api_v1_verify_email_otp_request_post()

Request email verification OTP

Generates and sends a 6-digit OTP to verify the authenticated user's email

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
    api_instance = moddns.VerificationApi(api_client)

    try:
        # Request email verification OTP
        api_instance.api_v1_verify_email_otp_request_post()
    except Exception as e:
        print("Exception when calling VerificationApi->api_v1_verify_email_otp_request_post: %s\n" % e)
```



### Parameters

This endpoint does not need any parameter.

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
**401** | Unauthorized |  -  |
**429** | Too Many Requests |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **api_v1_verify_reset_password_post**
> api_v1_verify_reset_password_post(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods)

Confirm password reset

Confirm password reset

### Example


```python
import moddns
from moddns.models.requests_confirm_reset_password_body import RequestsConfirmResetPasswordBody
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
    api_instance = moddns.VerificationApi(api_client)
    body = moddns.RequestsConfirmResetPasswordBody() # RequestsConfirmResetPasswordBody | Confirm password reset request
    x_mfa_code = 'x_mfa_code_example' # str | MFA OTP code (optional)
    x_mfa_methods = ['x_mfa_methods_example'] # List[str] | MFA methods (optional)

    try:
        # Confirm password reset
        api_instance.api_v1_verify_reset_password_post(body, x_mfa_code=x_mfa_code, x_mfa_methods=x_mfa_methods)
    except Exception as e:
        print("Exception when calling VerificationApi->api_v1_verify_reset_password_post: %s\n" % e)
```



### Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**RequestsConfirmResetPasswordBody**](RequestsConfirmResetPasswordBody.md)| Confirm password reset request | 
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
**401** | Unauthorized |  -  |
**500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)


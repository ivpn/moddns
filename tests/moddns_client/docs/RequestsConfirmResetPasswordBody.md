# RequestsConfirmResetPasswordBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**new_password** | **str** |  | 
**otp** | **str** |  | [optional] 
**token** | **str** |  | 

## Example

```python
from moddns.models.requests_confirm_reset_password_body import RequestsConfirmResetPasswordBody

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsConfirmResetPasswordBody from a JSON string
requests_confirm_reset_password_body_instance = RequestsConfirmResetPasswordBody.from_json(json)
# print the JSON string representation of the object
print(RequestsConfirmResetPasswordBody.to_json())

# convert the object into a dict
requests_confirm_reset_password_body_dict = requests_confirm_reset_password_body_instance.to_dict()
# create an instance of RequestsConfirmResetPasswordBody from a dict
requests_confirm_reset_password_body_from_dict = RequestsConfirmResetPasswordBody.from_dict(requests_confirm_reset_password_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



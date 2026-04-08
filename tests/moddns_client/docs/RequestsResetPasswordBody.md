# RequestsResetPasswordBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | 

## Example

```python
from moddns.models.requests_reset_password_body import RequestsResetPasswordBody

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsResetPasswordBody from a JSON string
requests_reset_password_body_instance = RequestsResetPasswordBody.from_json(json)
# print the JSON string representation of the object
print(RequestsResetPasswordBody.to_json())

# convert the object into a dict
requests_reset_password_body_dict = requests_reset_password_body_instance.to_dict()
# create an instance of RequestsResetPasswordBody from a dict
requests_reset_password_body_from_dict = RequestsResetPasswordBody.from_dict(requests_reset_password_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



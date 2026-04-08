# ApiRegisterAccountBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**email** | **str** |  | 
**password** | **str** |  | 
**subid** | **str** |  | 

## Example

```python
from moddns.models.api_register_account_body import ApiRegisterAccountBody

# TODO update the JSON string below
json = "{}"
# create an instance of ApiRegisterAccountBody from a JSON string
api_register_account_body_instance = ApiRegisterAccountBody.from_json(json)
# print the JSON string representation of the object
print(ApiRegisterAccountBody.to_json())

# convert the object into a dict
api_register_account_body_dict = api_register_account_body_instance.to_dict()
# create an instance of ApiRegisterAccountBody from a dict
api_register_account_body_from_dict = ApiRegisterAccountBody.from_dict(api_register_account_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



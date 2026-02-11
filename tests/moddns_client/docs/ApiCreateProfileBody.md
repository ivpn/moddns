# ApiCreateProfileBody


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** |  | [optional] 

## Example

```python
from moddns.models.api_create_profile_body import ApiCreateProfileBody

# TODO update the JSON string below
json = "{}"
# create an instance of ApiCreateProfileBody from a JSON string
api_create_profile_body_instance = ApiCreateProfileBody.from_json(json)
# print the JSON string representation of the object
print(ApiCreateProfileBody.to_json())

# convert the object into a dict
api_create_profile_body_dict = api_create_profile_body_instance.to_dict()
# create an instance of ApiCreateProfileBody from a dict
api_create_profile_body_from_dict = ApiCreateProfileBody.from_dict(api_create_profile_body_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



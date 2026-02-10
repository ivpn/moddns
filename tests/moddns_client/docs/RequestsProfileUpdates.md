# RequestsProfileUpdates


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**updates** | [**List[ModelProfileUpdate]**](ModelProfileUpdate.md) |  | 

## Example

```python
from moddns.models.requests_profile_updates import RequestsProfileUpdates

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsProfileUpdates from a JSON string
requests_profile_updates_instance = RequestsProfileUpdates.from_json(json)
# print the JSON string representation of the object
print(RequestsProfileUpdates.to_json())

# convert the object into a dict
requests_profile_updates_dict = requests_profile_updates_instance.to_dict()
# create an instance of RequestsProfileUpdates from a dict
requests_profile_updates_from_dict = RequestsProfileUpdates.from_dict(requests_profile_updates_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



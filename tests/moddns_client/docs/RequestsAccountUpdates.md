# RequestsAccountUpdates


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**updates** | [**List[ModelAccountUpdate]**](ModelAccountUpdate.md) |  | 

## Example

```python
from moddns.models.requests_account_updates import RequestsAccountUpdates

# TODO update the JSON string below
json = "{}"
# create an instance of RequestsAccountUpdates from a JSON string
requests_account_updates_instance = RequestsAccountUpdates.from_json(json)
# print the JSON string representation of the object
print(RequestsAccountUpdates.to_json())

# convert the object into a dict
requests_account_updates_dict = requests_account_updates_instance.to_dict()
# create an instance of RequestsAccountUpdates from a dict
requests_account_updates_from_dict = RequestsAccountUpdates.from_dict(requests_account_updates_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



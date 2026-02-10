# ModelSubscription


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active_until** | **str** |  | [optional] 
**type** | [**ModelSubscriptionType**](ModelSubscriptionType.md) |  | [optional] 

## Example

```python
from moddns.models.model_subscription import ModelSubscription

# TODO update the JSON string below
json = "{}"
# create an instance of ModelSubscription from a JSON string
model_subscription_instance = ModelSubscription.from_json(json)
# print the JSON string representation of the object
print(ModelSubscription.to_json())

# convert the object into a dict
model_subscription_dict = model_subscription_instance.to_dict()
# create an instance of ModelSubscription from a dict
model_subscription_from_dict = ModelSubscription.from_dict(model_subscription_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



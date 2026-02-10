# ModelStatisticsAggregated


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**total** | **int** | Note: \&quot;total\&quot; needs to be the same as in the repository mongo query | [optional] 

## Example

```python
from moddns.models.model_statistics_aggregated import ModelStatisticsAggregated

# TODO update the JSON string below
json = "{}"
# create an instance of ModelStatisticsAggregated from a JSON string
model_statistics_aggregated_instance = ModelStatisticsAggregated.from_json(json)
# print the JSON string representation of the object
print(ModelStatisticsAggregated.to_json())

# convert the object into a dict
model_statistics_aggregated_dict = model_statistics_aggregated_instance.to_dict()
# create an instance of ModelStatisticsAggregated from a dict
model_statistics_aggregated_from_dict = ModelStatisticsAggregated.from_dict(model_statistics_aggregated_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



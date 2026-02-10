# ModelBlocklist


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**blocklist_id** | **str** |  | 
**default** | **bool** | default blocklist is enabled when profile is created | [optional] 
**description** | **str** | displayed to the user | 
**entries** | **int** |  | [optional] 
**homepage** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**last_modified** | **str** |  | [optional] 
**name** | **str** | conventional blocklist name, displayed to the user | 
**source_url** | **str** |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from moddns.models.model_blocklist import ModelBlocklist

# TODO update the JSON string below
json = "{}"
# create an instance of ModelBlocklist from a JSON string
model_blocklist_instance = ModelBlocklist.from_json(json)
# print the JSON string representation of the object
print(ModelBlocklist.to_json())

# convert the object into a dict
model_blocklist_dict = model_blocklist_instance.to_dict()
# create an instance of ModelBlocklist from a dict
model_blocklist_from_dict = ModelBlocklist.from_dict(model_blocklist_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)



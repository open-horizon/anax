package plugin_registry

import ()

type PluginContext map[string]interface{}

func NewPluginContext() PluginContext {
	return make(map[string]interface{})
}

func (p PluginContext) Add(key string, val interface{}) {
	p[key] = val
}

func (p PluginContext) Get(key string) interface{} {
	if val, ok := p[key]; ok {
		return val
	}
	return nil
}

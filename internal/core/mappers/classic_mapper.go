package mappers

import (
	"github.com/jinzhu/copier"
)

type ClassicMapper[T, U any] struct {
}

func NewClassicMapper[T, U any]() *ClassicMapper[T, U] {
	return &ClassicMapper[T, U]{}
}

// ConvertModelToEntity converts a single model to an entity
func (m *ClassicMapper[T, U]) ConvertModelToEntity(model T) U {
	var entity U
	copier.Copy(&entity, model)
	return entity
}

// ConvertEntityToModel converts a single entity to a model
func (m *ClassicMapper[T, U]) ConvertEntityToModel(entity T) U {
	var model U
	copier.Copy(&model, entity)
	return model
}

// ConvertModelToEntities converts a slice of models to a slice of entities
func (m *ClassicMapper[T, U]) ConvertModelToEntities(models []T) []U {
	var entities []U
	for _, model := range models {
		var entity U
		copier.Copy(&entity, model)
		entities = append(entities, entity)
	}
	return entities
}

// ConvertEntitiesToModels converts a slice of entities to a slice of models
func (m *ClassicMapper[T, U]) ConvertEntitiesToModels(entities []T) []U {
	var models []U
	for _, entity := range entities {
		var model U
		copier.Copy(&model, entity)
		models = append(models, model)
	}
	return models
}

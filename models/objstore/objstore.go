package objstore

type ZipObjects struct {
	Name     *string  `form:"name"  validate:"omitempty,gt=0"`
	ObjectId []string `form:"object_id" validate:"gt=0"`
}

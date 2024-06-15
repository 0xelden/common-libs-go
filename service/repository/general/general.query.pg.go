package general

const (
	IncrementField = `
		UPDATE %s 
		SET %s = %s + @value, 
		updated_by = @by,
		updated_at = now()
		WHERE id = any(@id::uuid[])
	;`

	DecrementField = `
		UPDATE %s 
		SET %s = %s - @value, 
		updated_by = @by,
		updated_at = now()
		WHERE id = any(@id::uuid[])
	;`
)

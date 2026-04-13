import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  FormHelperText,
  HelperText,
  HelperTextItem,
  TextInput,
  ValidatedOptions,
} from '@patternfly/react-core';

interface PromptFieldProps {
  field: {
    name: string;
    description?: string;
    prompt?: string;
  };
  value: string;
  onChange: (value: string) => void;
}

export const PromptField: React.FC<PromptFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [validated, setValidated] = React.useState<ValidatedOptions>(ValidatedOptions.default);

  const handleValueChange = (_event: React.FormEvent<HTMLInputElement>, newValue: string) => {
    onChange(newValue);
    // Basic validation - required field should not be empty
    if (newValue.trim()) {
      setValidated(ValidatedOptions.success);
    } else {
      setValidated(ValidatedOptions.error);
    }
  };

  // Determine if this looks like a sensitive field based on name
  const isSensitive =
    field.name.toLowerCase().includes('password') ||
    field.name.toLowerCase().includes('secret') ||
    field.name.toLowerCase().includes('key') ||
    field.name.toLowerCase().includes('token');

  return (
    <>
      <TextInput
        id={`prompt-${field.name}`}
        type={isSensitive ? 'password' : 'text'}
        value={value}
        onChange={handleValueChange}
        placeholder={field.prompt || t('Enter value for {{fieldName}}', { fieldName: field.name })}
        validated={validated}
        isRequired
      />
      <FormHelperText>
        <HelperText>
          <HelperTextItem variant={validated === ValidatedOptions.error ? 'error' : 'default'}>
            {validated === ValidatedOptions.error
              ? t('This field is required')
              : field.description || t('Please provide a value for this required field')}
          </HelperTextItem>
        </HelperText>
      </FormHelperText>
    </>
  );
};

import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { FormHelperText, HelperText, HelperTextItem, TextInput } from '@patternfly/react-core';

interface StaticFieldProps {
  field: {
    name: string;
    description?: string;
    value?: string;
  };
  value: string;
  onChange: (value: string) => void;
}

export const StaticField: React.FC<StaticFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');

  React.useEffect(() => {
    if (field.value && !value) {
      onChange(field.value);
    }
  }, [field.value, value, onChange]);

  const displayValue = value || field.value || '';

  const handleValueChange = (_event: React.FormEvent<HTMLInputElement>, newValue: string) => {
    onChange(newValue);
  };

  return (
    <>
      <FormHelperText>
        <TextInput
          id={`static-${field.name}`}
          value={displayValue}
          isRequired={true}
          onChange={handleValueChange}
          placeholder={t('Static value')}
        />
        <HelperText>
          {field.description && <HelperTextItem>{field.description}</HelperTextItem>}
        </HelperText>
      </FormHelperText>
    </>
  );
};

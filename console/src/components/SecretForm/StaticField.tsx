import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Checkbox,
  FormHelperText,
  HelperText,
  HelperTextItem,
  TextInput,
} from '@patternfly/react-core';

interface StaticFieldProps {
  field: {
    name: string;
    description?: string;
    value?: string;
    override?: boolean;
  };
  value: string;
  onChange: (value: string) => void;
}

export const StaticField: React.FC<StaticFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__console-plugin-template');
  const [allowOverride, setAllowOverride] = React.useState(false);

  // Initialize with the static value if not already set
  React.useEffect(() => {
    if (field.value && !value) {
      onChange(field.value);
    }
  }, [field.value, value, onChange]);

  const displayValue = value || field.value || '';

  const handleOverrideChange = (checked: boolean) => {
    setAllowOverride(checked);
    if (!checked && field.value) {
      // Reset to original static value
      onChange(field.value);
    }
  };

  const handleValueChange = (_event: React.FormEvent<HTMLInputElement>, newValue: string) => {
    onChange(newValue);
  };

  const canOverride = field.override !== false; // Allow override unless explicitly disabled

  return (
    <>
      <TextInput
        id={`static-${field.name}`}
        value={displayValue}
        onChange={handleValueChange}
        isReadOnly={!allowOverride}
        placeholder={t('Static value')}
      />

      {canOverride && (
        <Checkbox
          id={`override-${field.name}`}
          label={t('Allow override')}
          isChecked={allowOverride}
          onChange={(_event, checked) => handleOverrideChange(checked)}
          style={{ marginTop: '8px' }}
        />
      )}

      <FormHelperText>
        <HelperText>
          <HelperTextItem>
            {allowOverride
              ? t('You can modify this pre-filled value')
              : t('This field has a pre-configured value')}
          </HelperTextItem>
          {field.description && <HelperTextItem>{field.description}</HelperTextItem>}
        </HelperText>
      </FormHelperText>
    </>
  );
};

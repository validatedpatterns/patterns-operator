import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Checkbox,
  FormHelperText,
  HelperText,
  HelperTextItem,
  TextInput,
} from '@patternfly/react-core';

interface GenerateFieldProps {
  field: {
    name: string;
    description?: string;
    vaultPolicy?: string;
  };
  value: string;
  onChange: (value: string) => void;
}

export const GenerateField: React.FC<GenerateFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__console-plugin-template');
  const [autoGenerate, setAutoGenerate] = React.useState(true);

  const handleAutoGenerateChange = (checked: boolean) => {
    setAutoGenerate(checked);
    if (checked) {
      onChange(''); // Clear manual value when switching to auto-generate
    }
  };

  const handleManualValueChange = (_event: React.FormEvent<HTMLInputElement>, newValue: string) => {
    onChange(newValue);
  };

  return (
    <>
      <Checkbox
        id={`auto-generate-${field.name}`}
        label={t('Auto-generate this value')}
        isChecked={autoGenerate}
        onChange={(_event, checked) => handleAutoGenerateChange(checked)}
      />
      {!autoGenerate && (
        <TextInput
          id={`manual-${field.name}`}
          type="password"
          value={value}
          onChange={handleManualValueChange}
          placeholder={t('Enter manual value')}
          style={{ marginTop: 'var(--pf-v6-global--spacer--sm)' }}
        />
      )}
      <FormHelperText>
        <HelperText>
          <HelperTextItem>
            {autoGenerate
              ? t('This value will be automatically generated using vault policies')
              : t('You can manually override the auto-generated value')}
          </HelperTextItem>
          {field.vaultPolicy && (
            <HelperTextItem>
              {t('Vault policy: {{policy}}', { policy: field.vaultPolicy })}
            </HelperTextItem>
          )}
          {field.description && <HelperTextItem>{field.description}</HelperTextItem>}
        </HelperText>
      </FormHelperText>
    </>
  );
};

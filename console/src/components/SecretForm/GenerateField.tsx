import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { Checkbox, HelperText, HelperTextItem, TextInput } from '@patternfly/react-core';

interface GenerateFieldProps {
  field: {
    name: string;
    description?: string;
    vaultPolicy?: string;
    override?: boolean;
  };
  value: string;
  onChange: (value: string) => void;
}

export const GenerateField: React.FC<GenerateFieldProps> = ({ field, value, onChange }) => {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const [autoGenerate, setAutoGenerate] = React.useState(true);
  const [allowOverride, setAllowOverride] = React.useState(false);

  const handleAutoGenerateChange = (checked: boolean) => {
    setAutoGenerate(checked);
    if (checked) {
      onChange('');
      setAllowOverride(false);
    }
  };

  const handleAllowOverrideChange = (checked: boolean) => {
    setAllowOverride(checked);
    if (!checked) {
      onChange('');
    }
  };

  const handleManualValueChange = (_event: React.FormEvent<HTMLInputElement>, newValue: string) => {
    onChange(newValue);
  };

  return (
    <>
      <HelperText>
        {field.description && <HelperTextItem>{field.description}</HelperTextItem>}
      </HelperText>
      <Checkbox
        id={`auto-generate-${field.name}`}
        label={t('Auto-generate this value')}
        isChecked={autoGenerate}
        onChange={(_event, checked) => handleAutoGenerateChange(checked)}
        body={t('If checked this value will be automatically generated using vault policies')}
        description={
          autoGenerate &&
          field.vaultPolicy &&
          t('Vault policy: {{policy}}', { policy: field.vaultPolicy })
        }
      />
      {autoGenerate && (
        <Checkbox
          id={`override-${field.name}`}
          label={t('Allow override')}
          isChecked={allowOverride}
          onChange={(_event, checked) => handleAllowOverrideChange(checked)}
          body={t(
            'If the secret already exists in the vault it will be changed if override is set to true',
          )}
        />
      )}
      {!autoGenerate && (
        <TextInput
          id={`manual-${field.name}`}
          type="password"
          value={value}
          onChange={handleManualValueChange}
          placeholder={t('Enter manual value')}
        />
      )}
    </>
  );
};

import { useState, type FormEvent } from 'react';
import { Button, Dialog, Flex, TextArea, TextField } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { putSecret, queryKeys } from '../lib/queries';
import { NAME_FORMAT_HINT, nameError } from '../lib/validate';
import { Alert } from './Alert';
import { FormField } from './FormField';
import { useToast } from './Toast';

interface AddSecretDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectName: string;
  envName: string;
  existingKeys: ReadonlySet<string>;
}

export function AddSecretDialog({
  open,
  onOpenChange,
  projectName,
  envName,
  existingKeys,
}: AddSecretDialogProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [key, setKey] = useState('');
  const [value, setValue] = useState('');
  // Split: keyError attaches to the key FormField (client-side validation
  // we can localize); formError is server-side and not field-specific.
  const [keyError, setKeyError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: ({ k, v }: { k: string; v: string }) => putSecret(projectName, envName, k, v),
    onSuccess: async (secret) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.secrets(projectName, envName) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.versions(projectName, envName) }),
      ]);
      toast.notify('success', `시크릿 "${secret.key}" 저장됨 (v${secret.version})`);
      setKey('');
      setValue('');
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      setFormError(
        err instanceof ApiError ? err.message : '저장에 실패했습니다. 잠시 후 다시 시도해 주세요.',
      );
    },
  });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setKeyError(null);
    setFormError(null);
    const trimmedKey = key.trim();
    const validation = nameError('key', trimmedKey);
    if (validation) {
      setKeyError(validation);
      return;
    }
    if (existingKeys.has(trimmedKey)) {
      setKeyError('같은 이름의 키가 이미 존재합니다. 표에서 직접 편집해 주세요.');
      return;
    }
    mutation.mutate({ k: trimmedKey, v: value });
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setKey('');
          setValue('');
          setKeyError(null);
          setFormError(null);
          mutation.reset();
        }
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="var(--dialog-width-md)">
        <Dialog.Title>새 시크릿</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          저장 즉시 새 버전이 생성되며 감사 로그에 기록됩니다.
        </Dialog.Description>
        <form onSubmit={onSubmit}>
          <Flex direction="column" gap="3">
            <FormField id="add-secret-key" label="키" hint={NAME_FORMAT_HINT} error={keyError}>
              {(fieldProps) => (
                <TextField.Root
                  {...fieldProps}
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  placeholder="예: DATABASE_URL"
                  autoFocus
                  spellCheck={false}
                />
              )}
            </FormField>
            <FormField id="add-secret-value" label="값" hint="저장 즉시 새 버전이 생성됩니다.">
              {(fieldProps) => (
                <TextArea
                  {...fieldProps}
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  rows={4}
                  spellCheck={false}
                />
              )}
            </FormField>
            <Alert variant="form" message={formError} />
            <Flex gap="3" justify="end">
              <Dialog.Close>
                <Button variant="soft" color="gray" type="button">
                  취소
                </Button>
              </Dialog.Close>
              <Button type="submit" disabled={mutation.isPending || key.trim() === ''}>
                {mutation.isPending ? '저장 중…' : '저장'}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}

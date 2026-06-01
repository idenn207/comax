import { useState, type FormEvent } from 'react';
import { Button, Dialog, Flex, TextField } from '@radix-ui/themes';
import { useMutation, useQueryClient } from '@tanstack/react-query';

import { ApiError } from '../lib/api';
import { createProject, queryKeys } from '../lib/queries';
import { NAME_FORMAT_HINT, nameError } from '../lib/validate';
import { Alert } from './Alert';
import { FormField } from './FormField';
import { useToast } from './Toast';

interface CreateProjectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateProjectDialog({ open, onOpenChange }: CreateProjectDialogProps) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [name, setName] = useState('');
  // Split mirrors AddSecret/CreateEnv: nameFieldError is the client-side
  // name validation that lives in the FormField; formError is the
  // server-side outcome (conflict / unknown / ApiError.message) and
  // mounts in the form-level Alert below the field. Keeping the two
  // separate keeps the "field-level vs form-level" vocabulary consistent
  // across all three dialogs.
  const [nameFieldError, setNameFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: (value: string) => createProject(value),
    onSuccess: async (project) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.projects() });
      toast.notify('success', `프로젝트 "${project.name}" 생성됨`);
      setName('');
      onOpenChange(false);
    },
    onError: (err: unknown) => {
      if (err instanceof ApiError) {
        if (err.code === 'conflict') {
          setFormError('같은 이름의 프로젝트가 이미 존재합니다.');
          return;
        }
        setFormError(err.message);
        return;
      }
      setFormError('알 수 없는 오류로 생성에 실패했습니다. 잠시 후 다시 시도해 주세요.');
    },
  });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setNameFieldError(null);
    setFormError(null);
    const trimmed = name.trim();
    const validation = nameError('project name', trimmed);
    if (validation) {
      setNameFieldError(validation);
      return;
    }
    mutation.mutate(trimmed);
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setName('');
          setNameFieldError(null);
          setFormError(null);
          mutation.reset();
        }
        onOpenChange(next);
      }}
    >
      <Dialog.Content maxWidth="var(--dialog-width-sm)">
        <Dialog.Title>새 프로젝트</Dialog.Title>
        <Dialog.Description size="2" mb="3">
          프로젝트는 환경(local/staging/prod 등)과 시크릿을 묶는 최상위 단위입니다.
        </Dialog.Description>
        <form onSubmit={onSubmit}>
          <Flex direction="column" gap="3">
            <FormField
              id="create-project-name"
              label="이름"
              hint={NAME_FORMAT_HINT}
              error={nameFieldError}
            >
              {(fieldProps) => (
                <TextField.Root
                  {...fieldProps}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="예: my-app"
                  autoFocus
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
              <Button type="submit" disabled={mutation.isPending || name.trim() === ''}>
                {mutation.isPending ? '생성 중…' : '생성'}
              </Button>
            </Flex>
          </Flex>
        </form>
      </Dialog.Content>
    </Dialog.Root>
  );
}

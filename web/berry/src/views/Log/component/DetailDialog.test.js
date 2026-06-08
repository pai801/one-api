import { render, screen, fireEvent } from '@testing-library/react';
import DetailDialog from './DetailDialog';

describe('DetailDialog', () => {
  const fullLogItem = {
    channel_name: 'ch1',
    request_header: '{"Authorization":"Bearer sk-xxx"}',
    request_body: '{"model":"gpt4","messages":[{"role":"user","content":"hello"}]}',
    response_body: '{"id":"resp1","choices":[{"text":"hi"}]}'
  };

  const minimalLogItem = {
    channel_name: 'ch1'
  };

  it('renders with all fields', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);
    expect(screen.getByText('详情')).toBeInTheDocument();
    expect(screen.getByText('ch1')).toBeInTheDocument();
    expect(screen.getByText('"model": "gpt4"', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('"id": "resp1"', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Bearer sk-xxx', { exact: false })).toBeInTheDocument();
  });

  it('hides empty fields', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={minimalLogItem} />);
    expect(screen.getByText('ch1')).toBeInTheDocument();
    expect(screen.queryByText(/request_header/i)).toBeNull();
    expect(screen.queryByText(/request_body/i)).toBeNull();
    expect(screen.queryByText(/response_body/i)).toBeNull();
  });

  it('does not render when closed', () => {
    render(<DetailDialog open={false} onClose={jest.fn()} logItem={minimalLogItem} />);
    expect(screen.queryByText('详情')).not.toBeInTheDocument();
  });

  it('shows copy buttons for request body and response body', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);
    const copyButtons = screen.getAllByRole('button', { name: /复制/ });
    expect(copyButtons).toHaveLength(2);
  });

  it('shows collapse buttons with 收起 text initially', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);
    const collapseButtons = screen.getAllByRole('button', { name: /收起/ });
    expect(collapseButtons).toHaveLength(2);
  });

  it('toggles expand/collapse state when clicking 收起/展开', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);

    const collapseButtons = screen.getAllByRole('button', { name: /收起/ });
    fireEvent.click(collapseButtons[0]);

    // One becomes 展开, the other remains 收起
    expect(screen.getAllByRole('button', { name: /展开/ })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: /收起/ })).toHaveLength(1);

    // Click expand to restore
    fireEvent.click(screen.getByRole('button', { name: /展开/ }));
    expect(screen.getAllByRole('button', { name: /收起/ })).toHaveLength(2);
  });

  it('applies max-height style to pre element when collapsed', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);

    fireEvent.click(screen.getAllByRole('button', { name: /收起/ })[0]);

    const pres = document.querySelectorAll('pre');
    // Order: 渠道名称 (0), 请求头 (1), 请求体 (2), 响应体 (3)
    // request body (index 2) should have max-height, response body (index 3) should not
    expect(pres[2].getAttribute('style')).toMatch(/max-height:\s*300px/);
    expect(pres[3].getAttribute('style')).toBeNull();
  });

  it('maintains independent collapse state between request body and response body', () => {
    render(<DetailDialog open={true} onClose={jest.fn()} logItem={fullLogItem} />);

    // Collapse request body only
    fireEvent.click(screen.getAllByRole('button', { name: /收起/ })[0]);

    // Response body should still show 收起
    expect(screen.getAllByRole('button', { name: /收起/ })).toHaveLength(1);

    // Expand request body back
    fireEvent.click(screen.getByRole('button', { name: /展开/ }));
    expect(screen.getAllByRole('button', { name: /收起/ })).toHaveLength(2);
  });
});

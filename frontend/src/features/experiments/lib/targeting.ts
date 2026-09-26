export type TargetingNode =
  | { type: 'or'; left: TargetingNode; right: TargetingNode }
  | { type: 'and'; left: TargetingNode; right: TargetingNode }
  | { type: 'not'; child: TargetingNode }
  | { type: 'cmp'; field: string; operator: string; value: TargetingLiteral }
  | { type: 'in'; field: string; values: TargetingLiteral[]; negated: boolean };

type TargetingLiteral = string | number | boolean | null | TargetingLiteral[];

type TokenKind =
  | 'eof'
  | 'ident'
  | 'string'
  | 'number'
  | 'bool'
  | 'null'
  | 'lparen'
  | 'rparen'
  | 'lbracket'
  | 'rbracket'
  | 'comma'
  | 'op'
  | 'and'
  | 'or'
  | 'not'
  | 'in';

interface Token {
  kind: TokenKind;
  text: string;
  value: unknown;
}

class TargetingSyntaxError extends Error {}

function isIdentStart(ch: string): boolean {
  return /[A-Za-z_.$]/.test(ch);
}

function isIdentPart(ch: string): boolean {
  return /[A-Za-z0-9_.$-]/.test(ch);
}

class Lexer {
  private pos = 0;
  private readonly input: string;

  constructor(input: string) {
    this.input = input;
  }

  tokenize(): Token[] {
    const tokens: Token[] = [];
    for (;;) {
      const token = this.next();
      tokens.push(token);
      if (token.kind === 'eof') {
        return tokens;
      }
    }
  }

  private charAt(pos: number): string {
    return this.input.charAt(pos);
  }

  private skipSpace(): void {
    while (this.pos < this.input.length && /\s/.test(this.charAt(this.pos))) {
      this.pos += 1;
    }
  }

  private next(): Token {
    this.skipSpace();
    if (this.pos >= this.input.length) {
      return { kind: 'eof', text: '', value: null };
    }
    const ch = this.charAt(this.pos);
    switch (ch) {
      case '(':
        this.pos += 1;
        return { kind: 'lparen', text: '(', value: null };
      case ')':
        this.pos += 1;
        return { kind: 'rparen', text: ')', value: null };
      case '[':
        this.pos += 1;
        return { kind: 'lbracket', text: '[', value: null };
      case ']':
        this.pos += 1;
        return { kind: 'rbracket', text: ']', value: null };
      case ',':
        this.pos += 1;
        return { kind: 'comma', text: ',', value: null };
      case '"':
      case "'":
        return this.lexString();
      case '=':
      case '!':
      case '>':
      case '<':
        return this.lexOperator();
      default:
        if (ch === '-' || /[0-9]/.test(ch)) {
          return this.lexNumber();
        }
        if (isIdentStart(ch)) {
          return this.lexIdent();
        }
        throw new TargetingSyntaxError(`unexpected character ${JSON.stringify(ch)}`);
    }
  }

  private lexString(): Token {
    const quote = this.charAt(this.pos);
    this.pos += 1;
    let out = '';
    while (this.pos < this.input.length) {
      const ch = this.charAt(this.pos);
      if (ch === '\\') {
        this.pos += 1;
        if (this.pos >= this.input.length) {
          throw new TargetingSyntaxError('unterminated escape');
        }
        const esc = this.charAt(this.pos);
        if (esc === 'n') out += '\n';
        else if (esc === 't') out += '\t';
        else if (esc === 'r') out += '\r';
        else out += esc;
        this.pos += 1;
        continue;
      }
      if (ch === quote) {
        this.pos += 1;
        return { kind: 'string', text: out, value: out };
      }
      out += ch;
      this.pos += 1;
    }
    throw new TargetingSyntaxError('unterminated string');
  }

  private lexNumber(): Token {
    const start = this.pos;
    if (this.charAt(this.pos) === '-') {
      this.pos += 1;
    }
    while (this.pos < this.input.length && /[0-9.eE+-]/.test(this.charAt(this.pos))) {
      this.pos += 1;
    }
    const raw = this.input.slice(start, this.pos);
    const value = Number(raw);
    if (!Number.isFinite(value)) {
      throw new TargetingSyntaxError(`invalid number ${JSON.stringify(raw)}`);
    }
    return { kind: 'number', text: raw, value };
  }

  private lexOperator(): Token {
    const ch = this.charAt(this.pos);
    this.pos += 1;
    let op = ch;
    if (this.pos < this.input.length && this.charAt(this.pos) === '=') {
      op += '=';
      this.pos += 1;
    }
    if (op === '=') {
      return { kind: 'op', text: '==', value: null };
    }
    if (op === '==' || op === '!=' || op === '>' || op === '>=' || op === '<' || op === '<=') {
      return { kind: 'op', text: op, value: null };
    }
    throw new TargetingSyntaxError(`invalid operator ${JSON.stringify(op)}`);
  }

  private lexIdent(): Token {
    const start = this.pos;
    while (this.pos < this.input.length && isIdentPart(this.charAt(this.pos))) {
      this.pos += 1;
    }
    const text = this.input.slice(start, this.pos);
    switch (text.toLowerCase()) {
      case 'and':
        return { kind: 'and', text, value: null };
      case 'or':
        return { kind: 'or', text, value: null };
      case 'not':
        return { kind: 'not', text, value: null };
      case 'in':
        return { kind: 'in', text, value: false };
      case 'notin':
        return { kind: 'in', text, value: true };
      case 'true':
        return { kind: 'bool', text, value: true };
      case 'false':
        return { kind: 'bool', text, value: false };
      case 'null':
      case 'nil':
        return { kind: 'null', text, value: null };
      default:
        return { kind: 'ident', text, value: null };
    }
  }
}

class Parser {
  private pos = 0;
  private readonly tokens: Token[];

  constructor(tokens: Token[]) {
    this.tokens = tokens;
  }

  parse(): TargetingNode {
    const node = this.parseOr();
    if (this.peek().kind !== 'eof') {
      throw new TargetingSyntaxError(`unexpected token ${JSON.stringify(this.peek().text)}`);
    }
    return node;
  }

  private peek(): Token {
    return this.tokens[this.pos] ?? { kind: 'eof', text: '', value: null };
  }

  private advance(): Token {
    const token = this.peek();
    this.pos += 1;
    return token;
  }

  private parseOr(): TargetingNode {
    let left = this.parseAnd();
    while (this.peek().kind === 'or') {
      this.advance();
      left = { type: 'or', left, right: this.parseAnd() };
    }
    return left;
  }

  private parseAnd(): TargetingNode {
    let left = this.parseUnary();
    while (this.peek().kind === 'and') {
      this.advance();
      left = { type: 'and', left, right: this.parseUnary() };
    }
    return left;
  }

  private parseUnary(): TargetingNode {
    if (this.peek().kind === 'not') {
      this.advance();
      return { type: 'not', child: this.parseUnary() };
    }
    return this.parsePrimary();
  }

  private parsePrimary(): TargetingNode {
    if (this.peek().kind === 'lparen') {
      this.advance();
      const node = this.parseOr();
      if (this.peek().kind !== 'rparen') {
        throw new TargetingSyntaxError("expected ')'");
      }
      this.advance();
      return node;
    }
    return this.parseComparison();
  }

  private parseComparison(): TargetingNode {
    const field = this.peek();
    if (field.kind !== 'ident') {
      throw new TargetingSyntaxError('expected field');
    }
    this.advance();
    const op = this.peek();
    if (op.kind === 'op') {
      this.advance();
      return { type: 'cmp', field: field.text, operator: op.text, value: this.parseLiteral() };
    }
    if (op.kind === 'in') {
      this.advance();
      return {
        type: 'in',
        field: field.text,
        values: this.parseArray(),
        negated: op.value === true,
      };
    }
    if (op.kind === 'not') {
      this.advance();
      if (this.peek().kind !== 'in') {
        throw new TargetingSyntaxError('expected IN after NOT');
      }
      this.advance();
      return { type: 'in', field: field.text, values: this.parseArray(), negated: true };
    }
    throw new TargetingSyntaxError(`expected operator after ${JSON.stringify(field.text)}`);
  }

  private parseLiteral(): TargetingLiteral {
    const token = this.peek();
    switch (token.kind) {
      case 'string':
      case 'number':
      case 'bool':
        this.advance();
        return token.value as TargetingLiteral;
      case 'null':
        this.advance();
        return null;
      case 'lbracket':
        return this.parseArray();
      default:
        throw new TargetingSyntaxError('expected literal');
    }
  }

  private parseArray(): TargetingLiteral[] {
    if (this.peek().kind !== 'lbracket') {
      throw new TargetingSyntaxError("expected '['");
    }
    this.advance();
    const values: TargetingLiteral[] = [];
    if (this.peek().kind === 'rbracket') {
      this.advance();
      return values;
    }
    for (;;) {
      values.push(this.parseLiteral());
      const next = this.peek();
      if (next.kind === 'comma') {
        this.advance();
        continue;
      }
      if (next.kind === 'rbracket') {
        this.advance();
        return values;
      }
      throw new TargetingSyntaxError("expected ',' or ']'");
    }
  }
}

export function parseTargetingDsl(input: string): TargetingNode {
  return new Parser(new Lexer(input).tokenize()).parse();
}

export function isValidTargetingDsl(input: string): boolean {
  const trimmed = input.trim();
  if (trimmed.length === 0) {
    return true;
  }
  try {
    parseTargetingDsl(trimmed);
    return true;
  } catch {
    return false;
  }
}
